module github.com/google/exposure-notifications-server

go 1.15

require (
	cloud.google.com/go v0.65.0
	cloud.google.com/go/storage v1.11.0
	contrib.go.opencensus.io/exporter/ocagent v0.7.0
	contrib.go.opencensus.io/exporter/prometheus v0.2.0
	contrib.go.opencensus.io/exporter/stackdriver v0.13.4
	github.com/Azure/azure-pipeline-go v0.2.3 // indirect
	github.com/Azure/azure-sdk-for-go v46.0.0+incompatible
	github.com/Azure/azure-storage-blob-go v0.10.0
	github.com/Azure/go-autorest/autorest v0.11.4
	github.com/Azure/go-autorest/autorest/adal v0.9.2
	github.com/Azure/go-autorest/autorest/azure/auth v0.5.1 // indirect
	github.com/Azure/go-autorest/autorest/to v0.4.0 // indirect
	github.com/Azure/go-autorest/autorest/validation v0.3.0 // indirect
	github.com/DataDog/datadog-go v3.7.1+incompatible // indirect
	github.com/Jeffail/gabs/v2 v2.5.0 // indirect
	github.com/Masterminds/semver v1.5.0 // indirect
	github.com/StackExchange/wmi v0.0.0-20190523213315-cbe66965904d // indirect
	github.com/aliyun/alibaba-cloud-sdk-go v1.61.213 // indirect
	github.com/armon/go-proxyproto v0.0.0-20200108142055-f0b8253b1507 // indirect
	github.com/aws/aws-sdk-go v1.34.19
	github.com/census-instrumentation/opencensus-proto v0.3.0 // indirect
	github.com/circonus-labs/circonusllhist v0.1.4 // indirect
	github.com/client9/misspell v0.3.4
	github.com/containerd/continuity v0.0.0-20200710164510-efbc4488d8fe // indirect
	github.com/dgrijalva/jwt-go v3.2.0+incompatible
	github.com/gin-gonic/gin v1.6.3
	github.com/go-ole/go-ole v1.2.4 // indirect
	github.com/go-playground/validator/v10 v10.3.0 // indirect
	github.com/go-test/deep v1.0.6 // indirect
	github.com/golang-migrate/migrate/v4 v4.12.2
	github.com/golang/protobuf v1.4.2
	github.com/google/go-cmp v0.5.2
	github.com/google/mako v0.2.0
	github.com/google/uuid v1.1.2
	github.com/gorilla/websocket v1.4.2 // indirect
	github.com/grpc-ecosystem/grpc-gateway v1.14.8 // indirect
	github.com/hashicorp/go-hclog v0.13.0
	github.com/hashicorp/go-memdb v1.2.1 // indirect
	github.com/hashicorp/go-msgpack v1.1.5 // indirect
	github.com/hashicorp/go-multierror v1.1.0
	github.com/hashicorp/go-plugin v1.3.0 // indirect
	github.com/hashicorp/go-retryablehttp v0.6.7 // indirect
	github.com/hashicorp/raft v1.1.2 // indirect
	github.com/hashicorp/vault v1.2.1-0.20200522144850-6f72d4ff250f
	github.com/hashicorp/vault-plugin-secrets-kv v0.5.5
	github.com/hashicorp/vault/api v1.0.5-0.20200522144850-6f72d4ff250f
	github.com/hashicorp/vault/sdk v0.1.14-0.20200519221838-e0cfd64bc267
	github.com/hashicorp/yamux v0.0.0-20190923154419-df201c70410d // indirect
	github.com/jackc/pgproto3/v2 v2.0.4 // indirect
	github.com/jackc/pgx/v4 v4.8.1
	github.com/jefferai/jsonx v1.0.1 // indirect
	github.com/kelseyhightower/run v0.0.17
	github.com/keybase/go-crypto v0.0.0-20200123153347-de78d2cb44f4 // indirect
	github.com/kr/text v0.2.0 // indirect
	github.com/lib/pq v1.8.0 // indirect
	github.com/lstoll/awskms v0.0.0-20200603175638-a388516467f1
	github.com/mattn/go-colorable v0.1.7 // indirect
	github.com/mikehelmick/go-chaff v0.3.0
	github.com/mitchellh/cli v1.1.1 // indirect
	github.com/mitchellh/go-testing-interface v1.14.1 // indirect
	github.com/mitchellh/mapstructure v1.3.3
	github.com/niemeyer/pretty v0.0.0-20200227124842-a10e7caefd8e // indirect
	github.com/oklog/run v1.1.0 // indirect
	github.com/onsi/ginkgo v1.13.0 // indirect
	github.com/opencontainers/go-digest v1.0.0 // indirect
	github.com/oracle/oci-go-sdk v19.3.0+incompatible // indirect
	github.com/ory/dockertest v3.3.5+incompatible
	github.com/pierrec/lz4 v2.5.2+incompatible // indirect
	github.com/posener/complete v1.2.3 // indirect
	github.com/prometheus/common v0.13.0
	github.com/prometheus/statsd_exporter v0.18.0 // indirect
	github.com/rakutentech/jwk-go v1.0.1
	github.com/sethvargo/go-envconfig v0.3.1
	github.com/sethvargo/go-gcpkms v0.1.0
	github.com/sethvargo/go-retry v0.1.0
	github.com/sethvargo/go-signalcontext v0.1.0
	github.com/shirou/gopsutil v2.20.4+incompatible // indirect
	github.com/shopspring/decimal v0.0.0-20200419222939-1884f454f8ea // indirect
	github.com/spf13/pflag v1.0.5 // indirect
	github.com/stretchr/objx v0.3.0 // indirect
	github.com/stretchr/testify v1.6.1 // indirect
	github.com/tv42/httpunix v0.0.0-20191220191345-2ba4b9c3382c // indirect
	go.opencensus.io v0.22.4
	go.uber.org/zap v1.16.0
	golang.org/x/crypto v0.0.0-20200820211705-5c72a883971a // indirect
	golang.org/x/net v0.0.0-20200904194848-62affa334b73 // indirect
	golang.org/x/oauth2 v0.0.0-20200902213428-5d25da1a8d43
	golang.org/x/sys v0.0.0-20200826173525-f9321e4c35a6 // indirect
	golang.org/x/tools v0.0.0-20200908191908-acefd226e2cc
	google.golang.org/api v0.30.0
	google.golang.org/genproto v0.0.0-20200904004341-0bd0a958aa1d
	google.golang.org/grpc v1.31.1
	google.golang.org/protobuf v1.25.0
	gopkg.in/check.v1 v1.0.0-20200227125254-8fa46927fb4f // indirect
	gopkg.in/ini.v1 v1.56.0 // indirect
	gopkg.in/square/go-jose.v2 v2.5.1 // indirect
	honnef.co/go/tools v0.0.1-2020.1.5
)

replace github.com/jackc/puddle => github.com/jeremyfaller/puddle v1.1.2-0.20200821025810-91d0159cc97a
