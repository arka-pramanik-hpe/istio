// Copyright Istio Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package dependencies

import (
	"bytes"
	"fmt"
	"io"
	"os/exec"
	"runtime"
	"strings"
	"syscall"

	netns "github.com/containernetworking/plugins/pkg/ns"
	"github.com/spf13/viper"
	"golang.org/x/sys/unix"
	utilversion "k8s.io/apimachinery/pkg/util/version"

	"istio.io/istio/pkg/log"
)

func (r *RealDependencies) execute(cmd string, ignoreErrors bool, stdin io.Reader, args ...string) error {
	log.Infof("Running command: %s %s", cmd, strings.Join(args, " "))

	externalCommand := exec.Command(cmd, args...)
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	externalCommand.Stdout = stdout
	externalCommand.Stderr = stderr
	externalCommand.Stdin = stdin

	// Grab all viper config and propagate it as environment variables to the child process
	repl := strings.NewReplacer("-", "_")
	for _, k := range viper.AllKeys() {
		v := viper.Get(k)
		if v == nil {
			continue
		}
		externalCommand.Env = append(externalCommand.Env, fmt.Sprintf("%s=%v", strings.ToUpper(repl.Replace(k)), v))
	}
	err := externalCommand.Run()
	if len(stdout.String()) != 0 {
		log.Infof("Command output: \n%v", stdout.String())
	}

	if !ignoreErrors && len(stderr.Bytes()) != 0 {
		log.Errorf("Command error output: \n%v", stderr.String())
	}

	return err
}

var (
	// IptablesRestoreLocking is the version where locking and -w is added to iptables-restore
	IptablesRestoreLocking = utilversion.MustParseGeneric("1.6.2")
	// IptablesLockfileEnv is the version where XTABLES_LOCKFILE is added to iptables.
	IptablesLockfileEnv = utilversion.MustParseGeneric("1.8.6")
)

// runInSandbox builds a lightweight sandbox ("container") to build a suitable environment to run iptables commands in.
// This is used in CNI, where commands are executed from the host but from within the container network namespace.
// This puts us in somewhat unconventionally territory.
func runInSandbox(lockFile string, f func() error) error {
	chErr := make(chan error, 1)
	n, nerr := netns.GetCurrentNS()
	if nerr != nil {
		return fmt.Errorf("failed to get current namespace: %v", nerr)
	}
	// setupSandbox builds the sandbox.
	setupSandbox := func() error {
		// First, unshare the mount namespace. This allows us to create custom mounts without impacting the host
		if err := unix.Unshare(unix.CLONE_NEWNS); err != nil {
			return fmt.Errorf("failed to unshare to new mount namespace: %v", err)
		}
		if err := n.Set(); err != nil {
			return fmt.Errorf("failed to reset network namespace: %v", err)
		}
		// Remount / as a private mount so that our mounts do not impact outside the namespace
		// (see https://unix.stackexchange.com/questions/246312/why-is-my-bind-mount-visible-outside-its-mount-namespace).
		if err := unix.Mount("", "/", "", unix.MS_PRIVATE|unix.MS_REC, ""); err != nil {
			return fmt.Errorf("failed to remount /: %v", err)
		}
		// In CNI, we are running the pod network namespace, but the host filesystem. Locking the host is both useless and harmful,
		// as it opens the risk of lock contention with other node actors (such as kube-proxy), and isn't actually needed at all.
		// Older iptables cannot turn off the lock explicitly, so we hack around it...
		// Overwrite the lock file with the network namespace file (which is assumed to be unique).
		// We are setting the lockfile to `r.NetworkNamespace`.
		// /dev/null looks like a good option, but actually doesn't work (it will ensure only one actor can access it)
		if lockFile != "" {
			if err := mount(lockFile, "/run/xtables.lock"); err != nil {
				return fmt.Errorf("bind mount of %q failed: %v", lockFile, err)
			}
		}

		// In some setups, iptables can make remote network calls(!!). Since these come from a partially initialized pod network namespace,
		// these calls can be blocked (or NetworkPolicy, etc could block them anyways).
		// This is triggered by NSS, which allows various things to use arbitrary code to lookup configuration that typically comes from files.
		// In our case, the culprit is the `xt_owner` (`-m owner`) module in iptables calls the `passwd` service to lookup the user.
		// To disallow this, bindmount /dev/null over nsswitch.conf so this never happens.
		// This should be safe to do, even if the user has an nsswitch entry that would work fine: we always use a numeric ID
		// so the passwd lookup doesn't need to succeed at all for Istio to function.
		// Effectively, we want a mini-container. In fact, running in a real container would be ideal but it is hard to do portably.
		// See https://github.com/istio/istio/issues/48416 for a real world example of this case.
		if err := mount("/dev/null", "/etc/nsswitch.conf"); err != nil {
			return fmt.Errorf("bind mount to %q failed: %v", "/etc/nsswitch.conf", err)
		}
		return nil
	}

	executed := false
	// Once we call unshare(CLONE_NEWNS), we cannot undo it explicitly. Instead, we need to unshare on a specific thread,
	// then kill that thread when we are done (or rather, let Go runtime kill the thread).
	// Unfortunately, making a new thread breaks us out of the network namespace we entered previously, so we need to restore that as well
	go func() {
		chErr <- func() error {
			// We now have exclusive access to this thread. Once the goroutine exits without calling UnlockOSThread, the go runtime will kill the thread for us
			// Warning: Do not call UnlockOSThread! Notably, netns.Do does call this.
			runtime.LockOSThread()
			if err := setupSandbox(); err != nil {
				return err
			}
			// Mark we have actually run the command. This lets us distinguish from a failure in setupSandbox() vs f()
			executed = true
			return f()
		}()
	}()
	err := <-chErr
	if err != nil && !executed {
		// We failed to setup the environment. Now we go into best effort mode.
		// Users running into this may have IPTables lock used unexpectedly or make unexpected NSS calls.
		// This is to support environments with restrictive access (from SELinux, but possibly others) that block these calls
		// See https://github.com/istio/istio/issues/48746
		log.Warnf("failed to setup execution environment, attempting to continue anyways: %v", err)
		// Try to execute as-is
		return f()
	}
	// Otherwise, we did execute; return the error from that execution.
	return err
}

func mount(src, dst string) error {
	return syscall.Mount(src, dst, "", syscall.MS_BIND|syscall.MS_RDONLY, "")
}

func (r *RealDependencies) executeXTables(cmd string, ignoreErrors bool, stdin io.ReadSeeker, args ...string) error {
	mode := "without lock"
	var c *exec.Cmd
	_, isWriteCommand := XTablesWriteCmds[cmd]
	needLock := isWriteCommand && !r.IptablesVersion.NoLocks()
	run := func(c *exec.Cmd) error {
		return c.Run()
	}
	if r.CNIMode {
		c = exec.Command(cmd, args...)
		// In CNI, we are running the pod network namespace, but the host filesystem, so we need to do some tricks
		// Call our binary again, but with <original binary> "unshare (subcommand to trigger mounts)" --lock-file=<network namespace> <original command...>
		// We do not shell out and call `mount` since this and sh are not available on all systems
		var lockFile string
		if needLock {
			if r.IptablesVersion.version.LessThan(IptablesLockfileEnv) {
				mode = "without lock by mount and nss"
				lockFile = r.NetworkNamespace
			} else {
				mode = "without lock by env and nss"
				c.Env = append(c.Env, "XTABLES_LOCKFILE="+r.NetworkNamespace)
			}
		} else {
			mode = "without nss"
		}

		run = func(c *exec.Cmd) error {
			return runInSandbox(lockFile, func() error {
				return c.Run()
			})
		}
	} else {
		if needLock {
			// We want the lock. Wait up to 30s for it.
			args = append(args, "--wait=30")
			c = exec.Command(cmd, args...)
			log.Debugf("running with lock")
			mode = "with wait lock"
		} else {
			// No locking supported/needed, just run as is. Nothing special
			c = exec.Command(cmd, args...)
		}
	}

	log.Infof("Running command (%s): %s %s", mode, cmd, strings.Join(args, " "))
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	c.Stdout = stdout
	c.Stderr = stderr
	c.Stdin = stdin
	err := run(c)
	if len(stdout.String()) != 0 {
		log.Infof("Command output: \n%v", stdout.String())
	}

	// TODO Check naming and redirection logic
	if (err != nil || len(stderr.String()) != 0) && !ignoreErrors {
		stderrStr := stderr.String()

		// Transform to xtables-specific error messages with more useful and actionable hints.
		if err != nil {
			stderrStr = transformToXTablesErrorMessage(stderrStr, err)
		}

		log.Errorf("Command error output: %v", stderrStr)
	}

	return err
}
