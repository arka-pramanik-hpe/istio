#!/bin/bash

# Copyright 2019 Istio Authors
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#    http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

set -e

UPDATE_BRANCH=${UPDATE_BRANCH:-"release-1.19"}

SCRIPTPATH="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
ROOTDIR=$(dirname "${SCRIPTPATH}")
cd "${ROOTDIR}"

# Get the sha of top commit
# $1 = repo
function getSha() {
  local dir result
  dir=$(mktemp -d)
  git clone --depth=1 "https://github.com/istio/${1}.git" -b "${UPDATE_BRANCH}" "${dir}"

  result="$(cd "${dir}" && git rev-parse HEAD)"
  rm -rf "${dir}"

  echo "${result}"
}

make update-common

export GO111MODULE=on
go get -u "istio.io/api@${UPDATE_BRANCH}"
go get -u "istio.io/client-go@${UPDATE_BRANCH}"
go mod tidy

sed -i "s/^BUILDER_SHA=.*\$/BUILDER_SHA=$(getSha release-builder)/" prow/release-commit.sh
chmod +x prow/release-commit.sh
sed -i '/PROXY_REPO_SHA/,/lastStableSHA/ { s/"lastStableSHA":.*/"lastStableSHA": "'"$(getSha proxy)"'"/  }; /ZTUNNEL_REPO_SHA/,/lastStableSHA/ { s/"lastStableSHA":.*/"lastStableSHA": "'"$(getSha ztunnel)"'"/  }' istio.deps

# shellcheck disable=SC1001
LATEST_DEB11_DISTROLESS_SHA256=$(crane digest gcr.io/distroless/static-debian11 | awk -F\: '{print $2}')
sed -i -E "s/sha256:[a-z0-9]+/sha256:${LATEST_DEB11_DISTROLESS_SHA256}/g" docker/Dockerfile.distroless

# shellcheck disable=SC1001
LATEST_IPTABLES_DISTROLESS_SHA256=$(crane digest gcr.io/istio-release/iptables | awk -F\: '{print $2}')
sed -i -E "s/sha256:[a-z0-9]+/sha256:${LATEST_IPTABLES_DISTROLESS_SHA256}/g" pilot/docker/Dockerfile.proxyv2
sed -i -E "s/sha256:[a-z0-9]+/sha256:${LATEST_IPTABLES_DISTROLESS_SHA256}/g" pilot/docker/Dockerfile.ztunnel
