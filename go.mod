module istio.io/istio

go 1.16

replace github.com/spf13/viper => github.com/istio/viper v1.3.3-0.20190515210538-2789fed3109c

// Old version had no license
replace github.com/chzyer/logex => github.com/chzyer/logex v1.1.11-0.20170329064859-445be9e134b2

// Avoid pulling in incompatible libraries
replace github.com/docker/distribution => github.com/docker/distribution v0.0.0-20191216044856-a8371794149d

replace github.com/docker/docker => github.com/moby/moby v17.12.0-ce-rc1.0.20200618181300-9dc6525e6118+incompatible

// Client-go does not handle different versions of mergo due to some breaking changes - use the matching version
replace github.com/imdario/mergo => github.com/imdario/mergo v0.3.5

require (
	cloud.google.com/go/compute/metadata v0.2.3
	cloud.google.com/go/security v1.12.0
	contrib.go.opencensus.io/exporter/prometheus v0.3.0
	github.com/AdaLogics/go-fuzz-headers v0.0.0-20210715213245-6c3934b029d8
	github.com/Masterminds/sprig/v3 v3.2.3
	github.com/aws/aws-sdk-go v1.43.16
	github.com/cenkalti/backoff v2.2.1+incompatible
	github.com/census-instrumentation/opencensus-proto v0.4.1
	github.com/cheggaaa/pb/v3 v3.0.8
	github.com/cncf/udpa/go v0.0.0-20220112060539-c52dc94e7fbe
	github.com/cncf/xds/go v0.0.0-20230105202645-06c439db220b
	github.com/containernetworking/cni v1.1.1
	github.com/containernetworking/plugins v1.1.1
	github.com/coreos/go-oidc v2.2.1+incompatible
	github.com/davecgh/go-spew v1.1.1
	github.com/docker/distribution v2.8.1+incompatible
	github.com/envoyproxy/go-control-plane v0.10.3
	github.com/evanphx/json-patch v5.6.0+incompatible
	github.com/evanphx/json-patch/v5 v5.5.0
	github.com/fatih/color v1.12.0
	github.com/fsnotify/fsnotify v1.6.0
	github.com/ghodss/yaml v1.0.0
	github.com/gogo/protobuf v1.3.2
	github.com/golang/protobuf v1.5.2
	github.com/google/go-cmp v0.5.9
	github.com/google/gofuzz v1.2.0
	github.com/google/uuid v1.3.0
	github.com/gorilla/mux v1.8.0
	github.com/gorilla/websocket v1.4.2
	github.com/grpc-ecosystem/go-grpc-middleware v1.3.0
	github.com/grpc-ecosystem/go-grpc-prometheus v1.2.0
	github.com/hashicorp/go-multierror v1.1.1
	github.com/hashicorp/go-version v1.3.0
	github.com/hashicorp/golang-lru v0.5.4
	github.com/kr/pretty v0.2.1
	github.com/kylelemons/godebug v1.1.0
	github.com/lestrrat-go/jwx v1.2.0
	github.com/lucas-clemente/quic-go v0.21.1
	github.com/mattn/go-isatty v0.0.13
	github.com/miekg/dns v1.1.42
	github.com/mitchellh/copystructure v1.2.0
	github.com/mitchellh/go-homedir v1.1.0
	github.com/onsi/gomega v1.23.0
	github.com/openshift/api v0.0.0-20200713203337-b2494ecb17dd
	github.com/pkg/errors v0.9.1
	github.com/pmezard/go-difflib v1.0.0
	github.com/prometheus/client_golang v1.14.0
	github.com/prometheus/client_model v0.3.0
	github.com/prometheus/common v0.37.0
	github.com/prometheus/prometheus v2.5.0+incompatible
	github.com/ryanuber/go-glob v1.0.0
	github.com/spf13/cobra v1.6.1
	github.com/spf13/pflag v1.0.5
	github.com/spf13/viper v1.8.1
	github.com/stretchr/testify v1.8.1
	github.com/xeipuuv/gojsonpointer v0.0.0-20190905194746-02993c407bfb // indirect
	github.com/yl2chen/cidranger v1.0.2
	go.opencensus.io v0.24.0
	go.uber.org/atomic v1.7.0
	go.uber.org/multierr v1.7.0
	golang.org/x/net v0.6.0
	golang.org/x/oauth2 v0.5.0
	golang.org/x/sync v0.1.0
	golang.org/x/sys v0.5.0
	golang.org/x/time v0.1.0
	gomodules.xyz/jsonpatch/v2 v2.2.0
	gomodules.xyz/jsonpatch/v3 v3.0.1
	google.golang.org/api v0.110.0
	google.golang.org/genproto v0.0.0-20230209215440-0dfe4f8abfcc
	google.golang.org/grpc v1.53.0
	google.golang.org/protobuf v1.28.1
	gopkg.in/square/go-jose.v2 v2.5.1
	gopkg.in/yaml.v2 v2.4.0
	gopkg.in/yaml.v3 v3.0.1
	helm.sh/helm/v3 v3.11.1
	istio.io/api v0.0.0-20220304045306-249321c725a9
	istio.io/client-go v1.11.7-0.20220304051621-641e42f0cee1
	istio.io/gogo-genproto v0.0.0-20220304045822-26ff2363d64b
	istio.io/pkg v0.0.0-20220304045307-c5ff97a4cef4
	k8s.io/api v0.26.0
	k8s.io/apiextensions-apiserver v0.26.0
	k8s.io/apimachinery v0.26.0
	k8s.io/cli-runtime v0.26.0
	k8s.io/client-go v0.26.0
	k8s.io/klog/v2 v2.80.1
	k8s.io/kube-openapi v0.0.0-20221012153701-172d655c2280
	k8s.io/kubectl v0.26.0
	k8s.io/utils v0.0.0-20221107191617-1a15be271d1d
	sigs.k8s.io/controller-runtime v0.9.0-beta.5
	sigs.k8s.io/gateway-api v0.3.0
	sigs.k8s.io/mcs-api v0.1.0
	sigs.k8s.io/yaml v1.3.0
)
