module github.com/google/exposure-notifications-server

go 1.15

require (
	cloud.google.com/go v0.74.0
	cloud.google.com/go/storage v1.12.0
	contrib.go.opencensus.io/exporter/ocagent v0.7.1-0.20200615190824-f8c219d2d895
	contrib.go.opencensus.io/exporter/prometheus v0.2.1-0.20200609204449-6bcf6f8577f0
	contrib.go.opencensus.io/exporter/stackdriver v0.13.5
	contrib.go.opencensus.io/integrations/ocsql v0.1.7
	github.com/Azure/azure-sdk-for-go v49.2.0+incompatible
	github.com/Azure/azure-storage-blob-go v0.12.0
	github.com/Azure/go-autorest/autorest v0.11.15
	github.com/Azure/go-autorest/autorest/adal v0.9.10
	github.com/Azure/go-autorest/autorest/azure/auth v0.5.5 // indirect
	github.com/Azure/go-autorest/autorest/validation v0.3.1 // indirect
	github.com/DataDog/datadog-go v3.7.1+incompatible // indirect
	github.com/Jeffail/gabs/v2 v2.5.0 // indirect
	github.com/Masterminds/semver v1.5.0 // indirect
	github.com/Microsoft/go-winio v0.4.16 // indirect
	github.com/alecthomas/units v0.0.0-20201120081800-1786d5ef83d4 // indirect
	github.com/aliyun/alibaba-cloud-sdk-go v1.61.213 // indirect
	github.com/armon/go-proxyproto v0.0.0-20200108142055-f0b8253b1507 // indirect
	github.com/asaskevich/govalidator v0.0.0-20200108200545-475eaeb16496 // indirect
	github.com/aws/aws-sdk-go v1.36.16
	github.com/census-instrumentation/opencensus-proto v0.3.0 // indirect
	github.com/circonus-labs/circonusllhist v0.1.4 // indirect
	github.com/client9/misspell v0.3.4
	github.com/containerd/continuity v0.0.0-20201208142359-180525291bb7 // indirect
	github.com/dgrijalva/jwt-go v3.2.0+incompatible
	github.com/dimchansky/utfbom v1.1.1 // indirect
	github.com/gin-gonic/gin v1.6.3
	github.com/go-playground/validator/v10 v10.4.1 // indirect
	github.com/golang-migrate/migrate/v4 v4.14.1
	github.com/golang/protobuf v1.4.3
	github.com/golang/snappy v0.0.2 // indirect
	github.com/google/go-cmp v0.5.4
	github.com/google/mako v0.2.0
	github.com/google/uuid v1.1.2
	github.com/gorilla/mux v1.8.0
	github.com/gostaticanalysis/analysisutil v0.6.1 // indirect
	github.com/grpc-ecosystem/grpc-gateway v1.16.0 // indirect
	github.com/hashicorp/errwrap v1.1.0 // indirect
	github.com/hashicorp/go-hclog v0.14.1
	github.com/hashicorp/go-memdb v1.2.1 // indirect
	github.com/hashicorp/go-msgpack v1.1.5 // indirect
	github.com/hashicorp/go-multierror v1.1.0
	github.com/hashicorp/go-plugin v1.3.0 // indirect
	github.com/hashicorp/go-retryablehttp v0.6.8 // indirect
	github.com/hashicorp/hcl/v2 v2.8.1
	github.com/hashicorp/vault v1.6.1
	github.com/hashicorp/vault-plugin-secrets-kv v0.7.0
	github.com/hashicorp/vault/api v1.0.5-0.20201001211907-38d91b749c77
	github.com/hashicorp/vault/sdk v0.1.14-0.20201214222404-d8fffe05d2f4
	github.com/hashicorp/yamux v0.0.0-20190923154419-df201c70410d // indirect
	github.com/jackc/pgx/v4 v4.10.0
	github.com/jefferai/jsonx v1.0.1 // indirect
	github.com/kelseyhightower/run v0.0.17
	github.com/keybase/go-crypto v0.0.0-20200123153347-de78d2cb44f4 // indirect
	github.com/leodido/go-urn v1.2.1 // indirect
	github.com/lib/pq v1.9.0 // indirect
	github.com/lstoll/awskms v0.0.0-20200603175638-a388516467f1
	github.com/mattn/go-colorable v0.1.7 // indirect
	github.com/mikehelmick/go-chaff v0.4.1
	github.com/mitchellh/go-testing-interface v1.14.1 // indirect
	github.com/mitchellh/mapstructure v1.4.0
	github.com/oklog/run v1.1.0 // indirect
	github.com/onsi/ginkgo v1.13.0 // indirect
	github.com/oracle/oci-go-sdk v19.3.0+incompatible // indirect
	github.com/ory/dockertest v3.3.5+incompatible
	github.com/pierrec/lz4 v2.6.0+incompatible // indirect
	github.com/posener/complete v1.2.3 // indirect
	github.com/prometheus/client_golang v1.9.0 // indirect
	github.com/prometheus/common v0.15.0
	github.com/prometheus/statsd_exporter v0.18.0 // indirect
	github.com/rakutentech/jwk-go v1.0.1
	github.com/sethvargo/go-envconfig v0.3.2
	github.com/sethvargo/go-gcpkms v0.1.0
	github.com/sethvargo/go-limiter v0.6.0 // indirect
	github.com/sethvargo/go-retry v0.1.0
	github.com/sethvargo/go-signalcontext v0.1.0
	github.com/sethvargo/zapw v0.1.0
	github.com/shirou/gopsutil v3.20.12-0.20201210134652-afe0c04c5d5a+incompatible // indirect
	github.com/smartystreets/assertions v1.0.0 // indirect
	github.com/stretchr/objx v0.3.0 // indirect
	github.com/timakin/bodyclose v0.0.0-20200424151742-cb6215831a94
	github.com/tv42/httpunix v0.0.0-20191220191345-2ba4b9c3382c // indirect
	github.com/ugorji/go v1.2.1 // indirect
	go.opencensus.io v0.22.5
	go.uber.org/multierr v1.6.0 // indirect
	go.uber.org/zap v1.16.0
	golang.org/x/crypto v0.0.0-20201221181555-eec23a3978ad // indirect
	golang.org/x/net v0.0.0-20201224014010-6772e930b67b // indirect
	golang.org/x/oauth2 v0.0.0-20201208152858-08078c50e5b5
	golang.org/x/sync v0.0.0-20201207232520-09787c993a3a // indirect
	golang.org/x/sys v0.0.0-20201223074533-0d417f636930 // indirect
	golang.org/x/time v0.0.0-20201208040808-7e3f01d25324
	golang.org/x/tools v0.0.0-20201228204837-84d76fe3206d
	google.golang.org/api v0.36.0
	google.golang.org/genproto v0.0.0-20201214200347-8c77b98c765d
	google.golang.org/grpc v1.34.0
	google.golang.org/protobuf v1.25.0
	gopkg.in/ini.v1 v1.56.0 // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
	gopkg.in/yaml.v3 v3.0.0-20200615113413-eeeca48fe776 // indirect
	honnef.co/go/tools v0.1.0
	k8s.io/api v0.18.7-rc.0 // indirect
	sigs.k8s.io/structured-merge-diff/v3 v3.0.1-0.20200706213357-43c19bbb7fba // indirect
)
