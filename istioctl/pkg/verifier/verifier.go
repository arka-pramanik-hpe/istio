// Copyright Istio Authors.
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

package verifier

import (
	"context"
	"fmt"
	"strings"

	"github.com/fatih/color"
	"github.com/hashicorp/go-multierror"
	admitv1 "k8s.io/api/admissionregistration/v1"
	appsv1 "k8s.io/api/apps/v1"
	v1batch "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	apimachinery_schema "k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/cli-runtime/pkg/resource"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/yaml"

	"istio.io/api/label"
	"istio.io/istio/istioctl/pkg/clioptions"
	operator_istio "istio.io/istio/operator/pkg/apis/istio"
	"istio.io/istio/operator/pkg/apis/istio/v1alpha1"
	"istio.io/istio/operator/pkg/controlplane"
	"istio.io/istio/operator/pkg/manifest"
	"istio.io/istio/operator/pkg/translate"
	"istio.io/istio/operator/pkg/util"
	"istio.io/istio/operator/pkg/util/clog"
	"istio.io/istio/pkg/kube"
)

var (
	istioOperatorGVR = apimachinery_schema.GroupVersionResource{
		Group:    v1alpha1.SchemeGroupVersion.Group,
		Version:  v1alpha1.SchemeGroupVersion.Version,
		Resource: "istiooperators",
	}

	// specialKinds is a map of special kinds to their corresponding kind names, which do not follow the
	// standard convention of pluralizing the kind name.
	specialKinds = map[string]string{
		"NetworkAttachmentDefinition": "network-attachment-definitions",
	}
)

// StatusVerifier checks status of certain resources like deployment,
// jobs and also verifies count of certain resource types.
type StatusVerifier struct {
	istioNamespace   string
	manifestsPath    string
	filenames        []string
	controlPlaneOpts clioptions.ControlPlaneOptions
	logger           clog.Logger
	iop              *v1alpha1.IstioOperator
	successMarker    string
	failureMarker    string
	client           kube.CLIClient
}

type StatusVerifierOptions func(*StatusVerifier)

func WithLogger(l clog.Logger) StatusVerifierOptions {
	return func(s *StatusVerifier) {
		s.logger = l
	}
}

func WithIOP(iop *v1alpha1.IstioOperator) StatusVerifierOptions {
	return func(s *StatusVerifier) {
		s.iop = iop
	}
}

// NewStatusVerifier creates a new instance of post-install verifier
// which checks the status of various resources from the manifest.
func NewStatusVerifier(istioNamespace, manifestsPath, kubeconfig, context string,
	filenames []string, controlPlaneOpts clioptions.ControlPlaneOptions,
	options ...StatusVerifierOptions,
) (*StatusVerifier, error) {
	client, err := kube.NewCLIClient(kube.BuildClientCmd(kubeconfig, context), "")
	if err != nil {
		return nil, fmt.Errorf("failed to connect Kubernetes API server, error: %v", err)
	}

	verifier := StatusVerifier{
		logger:           clog.NewDefaultLogger(),
		successMarker:    "✔",
		failureMarker:    "✘",
		istioNamespace:   istioNamespace,
		manifestsPath:    manifestsPath,
		filenames:        filenames,
		controlPlaneOpts: controlPlaneOpts,
		client:           client,
	}

	for _, opt := range options {
		opt(&verifier)
	}

	return &verifier, nil
}

func (v *StatusVerifier) Colorize() {
	v.successMarker = color.New(color.FgGreen).Sprint(v.successMarker)
	v.failureMarker = color.New(color.FgRed).Sprint(v.failureMarker)
}

// Verify implements Verifier interface. Here we check status of deployment
// and jobs, count various resources for verification.
func (v *StatusVerifier) Verify() error {
	if v.iop != nil {
		return v.verifyFinalIOP()
	}
	if len(v.filenames) == 0 {
		return v.verifyInstallIOPRevision()
	}
	return v.verifyInstall()
}

func (v *StatusVerifier) verifyInstallIOPRevision() error {
	var err error
	if v.controlPlaneOpts.Revision == "" {
		v.controlPlaneOpts.Revision, err = v.getRevision()
		if err != nil {
			return err
		}
	} else if v.controlPlaneOpts.Revision == "default" {
		v.controlPlaneOpts.Revision = ""
	}
	iops, err := v.operatorsFromCluster(v.controlPlaneOpts.Revision)
	if err != nil {
		// At this point we know there is no IstioOperator defining a control plane.  This may
		// be the case in a Istio cluster with external control plane.
		v.logger.LogAndErrorf("error while fetching revision %s: %v", v.controlPlaneOpts.Revision, err.Error())
		injector, err2 := v.injectorFromCluster(v.controlPlaneOpts.Revision)
		if err2 == nil && injector != nil {
			// The cluster *is* configured for Istio, but no IOP is present.  This could mean
			// - the user followed our remote control plane instructions
			// - helm was used
			// - user did `istioctl manifest generate | kubectl apply ...`
			return fmt.Errorf("Istio present but verify-install needs an IstioOperator or manifest for comparison. Supply flag --filename <yaml>") // nolint: stylecheck
		}
		return fmt.Errorf("could not load IstioOperator from cluster: %v. Use --filename", err)
	}
	var crdTotal, istioDeploymentTotal, daemonSetTotal int
	multiErr := &multierror.Error{}
	for _, iop := range iops {
		if v.manifestsPath != "" {
			iop.Spec.InstallPackagePath = v.manifestsPath
		}
		profile := manifest.GetProfile(iop)
		by, err := yaml.Marshal(iop)
		if err != nil {
			return err
		}
		mergedIOP, err := manifest.GetMergedIOP(string(by), profile, v.manifestsPath, v.controlPlaneOpts.Revision, v.client, v.logger)
		if err != nil {
			return err
		}
		crdCount, istioDeploymentCount, daemonSetCount, err := v.verifyPostInstallIstioOperator(
			mergedIOP, fmt.Sprintf("in cluster operator %s", mergedIOP.GetName()))
		if err != nil {
			multiErr = multierror.Append(multiErr, err)
		}
		crdTotal += crdCount
		istioDeploymentTotal += istioDeploymentCount
		daemonSetTotal += daemonSetCount
	}
	return v.reportStatus(crdTotal, istioDeploymentTotal, daemonSetTotal, multiErr.ErrorOrNil())
}

func (v *StatusVerifier) getRevision() (string, error) {
	var revision string
	var revs string
	revCount := 0
	pods, err := v.client.PodsForSelector(context.TODO(), v.istioNamespace, "app=istiod")
	if err != nil {
		return "", fmt.Errorf("failed to fetch istiod pod, error: %v", err)
	}
	for _, pod := range pods.Items {
		rev := pod.ObjectMeta.GetLabels()[label.IoIstioRev.Name]
		revCount++
		if rev == "default" {
			continue
		}
		revision = rev
	}
	if revision == "" {
		revs = "default"
	} else {
		revs = revision
	}
	v.logger.LogAndPrintf("%d Istio control planes detected, checking --revision %q only", revCount, revs)
	return revision, nil
}

func (v *StatusVerifier) verifyFinalIOP() error {
	crdCount, istioDeploymentCount, daemonSetCount, err := v.verifyPostInstallIstioOperator(
		v.iop, fmt.Sprintf("IOP:%s", v.iop.GetName()))
	return v.reportStatus(crdCount, istioDeploymentCount, daemonSetCount, err)
}

func (v *StatusVerifier) verifyInstall() error {
	// This is not a pre-check.  Check that the supplied resources exist in the cluster
	r := resource.NewBuilder(v.client.UtilFactory()).
		Unstructured().
		FilenameParam(false, &resource.FilenameOptions{Filenames: v.filenames}).
		Flatten().
		Do()
	if r.Err() != nil {
		return r.Err()
	}
	visitor := genericclioptions.ResourceFinderForResult(r).Do()
	crdCount, istioDeploymentCount, generatedDaemonsets, err := v.verifyPostInstall(
		visitor, strings.Join(v.filenames, ","))
	return v.reportStatus(crdCount, istioDeploymentCount, generatedDaemonsets, err)
}

func (v *StatusVerifier) verifyPostInstallIstioOperator(iop *v1alpha1.IstioOperator, filename string) (int, int, int, error) {
	t := translate.NewTranslator()
	ver, err := v.client.GetKubernetesVersion()
	if err != nil {
		return 0, 0, 0, err
	}
	cp, err := controlplane.NewIstioControlPlane(iop.Spec, t, nil, ver)
	if err != nil {
		return 0, 0, 0, err
	}
	if err := cp.Run(); err != nil {
		return 0, 0, 0, err
	}

	manifests, errs := cp.RenderManifest()
	if len(errs) > 0 {
		return 0, 0, 0, errs.ToError()
	}

	builder := resource.NewBuilder(v.client.UtilFactory()).ContinueOnError().Unstructured()
	for cat, manifest := range manifests {
		for i, manitem := range manifest {
			reader := strings.NewReader(manitem)
			pseudoFilename := fmt.Sprintf("%s:%d generated from %s", cat, i, filename)
			builder = builder.Stream(reader, pseudoFilename)
		}
	}
	r := builder.Flatten().Do()
	if r.Err() != nil {
		return 0, 0, 0, r.Err()
	}
	visitor := genericclioptions.ResourceFinderForResult(r).Do()
	// Indirectly RECURSE back into verifyPostInstall with the manifest we just generated
	generatedCrds, generatedDeployments, generatedDaemonSets, err := v.verifyPostInstall(
		visitor,
		fmt.Sprintf("generated from %s", filename))
	if err != nil {
		return generatedCrds, generatedDeployments, generatedDaemonSets, err
	}

	return generatedCrds, generatedDeployments, generatedDaemonSets, nil
}

func (v *StatusVerifier) verifyPostInstall(visitor resource.Visitor, filename string) (int, int, int, error) {
	crdCount := 0
	istioDeploymentCount := 0
	daemonSetCount := 0
	err := visitor.Visit(func(info *resource.Info, err error) error {
		if err != nil {
			return err
		}
		content, err := runtime.DefaultUnstructuredConverter.ToUnstructured(info.Object)
		if err != nil {
			return err
		}
		un := &unstructured.Unstructured{Object: content}
		kind := un.GetKind()
		name := un.GetName()
		namespace := un.GetNamespace()
		kinds := resourceKinds(un)
		if namespace == "" {
			namespace = v.istioNamespace
		}
		switch kind {
		case "Deployment":
			deployment := &appsv1.Deployment{}
			err = info.Client.
				Get().
				Resource(kinds).
				Namespace(namespace).
				Name(name).
				VersionedParams(&metav1.GetOptions{}, scheme.ParameterCodec).
				Do(context.TODO()).
				Into(deployment)
			if err != nil {
				v.reportFailure(kind, name, namespace, err)
				return err
			}
			if err = verifyDeploymentStatus(deployment); err != nil {
				ivf := istioVerificationFailureError(filename, err)
				v.reportFailure(kind, name, namespace, ivf)
				return ivf
			}
			if namespace == v.istioNamespace && strings.HasPrefix(name, "istio") {
				istioDeploymentCount++
			}
		case "Job":
			job := &v1batch.Job{}
			err = info.Client.
				Get().
				Resource(kinds).
				Namespace(namespace).
				Name(name).
				VersionedParams(&metav1.GetOptions{}, scheme.ParameterCodec).
				Do(context.TODO()).
				Into(job)
			if err != nil {
				v.reportFailure(kind, name, namespace, err)
				return err
			}
			if err := verifyJobPostInstall(job); err != nil {
				ivf := istioVerificationFailureError(filename, err)
				v.reportFailure(kind, name, namespace, ivf)
				return ivf
			}
		case "IstioOperator":
			// It is not a problem if the cluster does not include the IstioOperator
			// we are checking.  Instead, verify the cluster has the things the
			// IstioOperator specifies it should have.

			// IstioOperator isn't part of pkg/config/schema/collections,
			// usual conversion not available.  Convert unstructured to string
			// and ask operator code to unmarshal.
			fixTimestampRelatedUnmarshalIssues(un)

			by := util.ToYAML(un)
			unmergedIOP, err := operator_istio.UnmarshalIstioOperator(by, true)
			if err != nil {
				v.reportFailure(kind, name, namespace, err)
				return err
			}
			profile := manifest.GetProfile(unmergedIOP)
			iop, err := manifest.GetMergedIOP(by, profile, v.manifestsPath, v.controlPlaneOpts.Revision,
				v.client, v.logger)
			if err != nil {
				v.reportFailure(kind, name, namespace, err)
				return err
			}
			if v.manifestsPath != "" {
				iop.Spec.InstallPackagePath = v.manifestsPath
			}
			if v1alpha1.Namespace(iop.Spec) == "" {
				v1alpha1.SetNamespace(iop.Spec, v.istioNamespace)
			}
			generatedCrds, generatedDeployments, generatedDaemonSets, err := v.verifyPostInstallIstioOperator(iop, filename)
			crdCount += generatedCrds
			istioDeploymentCount += generatedDeployments
			daemonSetCount += generatedDaemonSets
			if err != nil {
				return err
			}
		case "DaemonSet":
			ds := &appsv1.DaemonSet{}
			err = info.Client.
				Get().
				Resource(kinds).
				Namespace(namespace).
				Name(name).
				VersionedParams(&metav1.GetOptions{}, scheme.ParameterCodec).
				Do(context.TODO()).
				Into(ds)
			if err != nil {
				v.reportFailure(kind, name, namespace, err)
				return err
			}
			daemonSetCount++
			if err = verifyDaemonSetStatus(ds); err != nil {
				ivf := istioVerificationFailureError(filename, err)
				v.reportFailure(kind, name, namespace, ivf)
				return ivf
			}
		default:
			result := info.Client.
				Get().
				Resource(kinds).
				Name(name).
				Do(context.TODO())
			if result.Error() != nil {
				result = info.Client.
					Get().
					Resource(kinds).
					Namespace(namespace).
					Name(name).
					Do(context.TODO())
				if result.Error() != nil {
					v.reportFailure(kind, name, namespace, result.Error())
					return istioVerificationFailureError(filename,
						fmt.Errorf("the required %s:%s is not ready due to: %v",
							kind, name, result.Error()))
				}
			}
			if kind == "CustomResourceDefinition" {
				crdCount++
			}
		}
		v.logger.LogAndPrintf("%s %s: %s.%s checked successfully", v.successMarker, kind, name, namespace)
		return nil
	})
	return crdCount, istioDeploymentCount, daemonSetCount, err
}

func resourceKinds(un *unstructured.Unstructured) string {
	kinds := findResourceInSpec(un.GetObjectKind().GroupVersionKind())
	if kinds == "" {
		kinds = strings.ToLower(un.GetKind()) + "s"
	}
	// Override with special kind if it exists in the map
	if specialKind, exists := specialKinds[un.GetKind()]; exists {
		kinds = specialKind
	}
	return kinds
}

// Find Istio injector matching revision.  ("" matches any revision.)
func (v *StatusVerifier) injectorFromCluster(revision string) (*admitv1.MutatingWebhookConfiguration, error) {
	hooks, err := v.client.Kube().AdmissionregistrationV1().MutatingWebhookConfigurations().List(context.Background(), metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	revCount := 0
	var hookmatch *admitv1.MutatingWebhookConfiguration
	for i, hook := range hooks.Items {
		rev := hook.ObjectMeta.GetLabels()[label.IoIstioRev.Name]
		if rev != "" {
			revCount++
			revision = rev
			if revision == "" || revision == rev {
				hookmatch = &hooks.Items[i]
			}
		}
	}

	v.logger.LogAndPrintf("%d Istio injectors detected", revCount)
	if hookmatch != nil {
		return hookmatch, nil
	}

	return nil, fmt.Errorf("Istio injector revision %q not found", revision) // nolint: stylecheck
}

// Find an IstioOperator matching revision in the cluster.  The IstioOperators
// don't have a label for their revision, so we parse them and check .Spec.Revision
func (v *StatusVerifier) operatorsFromCluster(revision string) ([]*v1alpha1.IstioOperator, error) {
	iops, err := AllOperatorsInCluster(v.client.Dynamic())
	if err != nil {
		return nil, err
	}
	iopsMatch := make([]*v1alpha1.IstioOperator, 0)
	for _, iop := range iops {
		if iop.Spec.Revision == revision ||
			(iop.Spec.Revision == "default" && revision == "") ||
			(iop.Spec.Revision == "" && revision == "default") {
			iopsMatch = append(iopsMatch, iop)
		}
	}
	if len(iopsMatch) > 0 {
		return iopsMatch, nil
	}
	return nil, fmt.Errorf("control plane revision %q not found", revision)
}

func (v *StatusVerifier) reportStatus(crdCount, istioDeploymentCount, daemonSetCount int, err error) error {
	v.logger.LogAndPrintf("Checked %v custom resource definitions", crdCount)
	v.logger.LogAndPrintf("Checked %v Istio Deployments", istioDeploymentCount)
	if daemonSetCount > 0 {
		v.logger.LogAndPrintf("Checked %v Istio Daemonsets", daemonSetCount)
	}
	if istioDeploymentCount == 0 {
		if err != nil {
			v.logger.LogAndPrintf("! No Istio installation found: %v", err)
		} else {
			v.logger.LogAndPrintf("! No Istio installation found")
		}
		return fmt.Errorf("no Istio installation found")
	}
	if err != nil {
		// Don't return full error; it is usually an unwieldy aggregate
		return fmt.Errorf("Istio installation failed") // nolint
	}
	v.logger.LogAndPrintf("%s Istio is installed and verified successfully", v.successMarker)
	return nil
}

func fixTimestampRelatedUnmarshalIssues(un *unstructured.Unstructured) {
	un.SetCreationTimestamp(metav1.Time{}) // UnmarshalIstioOperator chokes on these

	// UnmarshalIstioOperator fails because managedFields could contain time
	// and gogo/protobuf/jsonpb(v1.3.1) tries to unmarshal it as struct (the type
	// meta_v1.Time is really a struct) and fails.
	un.SetManagedFields([]metav1.ManagedFieldsEntry{})
}

// Find all IstioOperator in the cluster.
func AllOperatorsInCluster(client dynamic.Interface) ([]*v1alpha1.IstioOperator, error) {
	ul, err := client.
		Resource(istioOperatorGVR).
		List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	retval := make([]*v1alpha1.IstioOperator, 0)
	for _, un := range ul.Items {
		fixTimestampRelatedUnmarshalIssues(&un)
		by := util.ToYAML(un.Object)
		iop, err := operator_istio.UnmarshalIstioOperator(by, true)
		if err != nil {
			return nil, err
		}
		retval = append(retval, iop)
	}
	return retval, nil
}

func istioVerificationFailureError(filename string, reason error) error {
	return fmt.Errorf("Istio installation failed, incomplete or does not match \"%s\": %v", filename, reason) // nolint
}

func (v *StatusVerifier) reportFailure(kind, name, namespace string, err error) {
	v.logger.LogAndPrintf("%s %s: %s.%s: %v", v.failureMarker, kind, name, namespace, err)
}
