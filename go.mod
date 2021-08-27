module github.com/google/exposure-notifications-server

go 1.16

require (
	cloud.google.com/go v0.87.0
	cloud.google.com/go/storage v1.16.0
	contrib.go.opencensus.io/exporter/ocagent v0.7.0
	contrib.go.opencensus.io/exporter/prometheus v0.3.0
	contrib.go.opencensus.io/exporter/stackdriver v0.13.8
	contrib.go.opencensus.io/integrations/ocsql v0.1.7
	github.com/Azure/azure-sdk-for-go v55.7.0+incompatible
	github.com/Azure/azure-storage-blob-go v0.14.0
	github.com/Azure/go-ansiterm v0.0.0-20210617225240-d185dfc1b5a1 // indirect
	github.com/Azure/go-autorest/autorest v0.11.19
	github.com/Azure/go-autorest/autorest/adal v0.9.14
	github.com/Azure/go-autorest/autorest/azure/auth v0.5.8 // indirect
	github.com/Azure/go-autorest/autorest/to v0.4.0 // indirect
	github.com/Azure/go-autorest/autorest/validation v0.3.1 // indirect
	github.com/Microsoft/go-winio v0.5.0 // indirect
	github.com/Nvveen/Gotty v0.0.0-20120604004816-cd527374f1e5 // indirect
	github.com/ashanbrown/forbidigo v1.2.0 // indirect
	github.com/ashanbrown/makezero v0.0.0-20210520155254-b6261585ddde // indirect
	github.com/aws/aws-sdk-go v1.40.2
	github.com/bombsimon/wsl/v3 v3.3.0 // indirect
	github.com/cenkalti/backoff/v3 v3.2.2 // indirect
	github.com/charithe/durationcheck v0.0.8 // indirect
	github.com/client9/misspell v0.3.4
	github.com/containerd/continuity v0.1.0 // indirect
	github.com/daixiang0/gci v0.2.9 // indirect
	github.com/esimonov/ifshort v1.0.2 // indirect
	github.com/form3tech-oss/jwt-go v3.2.3+incompatible // indirect
	github.com/gin-gonic/gin v1.7.2
	github.com/go-critic/go-critic v0.5.6 // indirect
	github.com/go-playground/validator/v10 v10.7.0 // indirect
	github.com/gofrs/flock v0.8.1 // indirect
	github.com/golang-jwt/jwt v3.2.1+incompatible
	github.com/golang-migrate/migrate/v4 v4.14.1
	github.com/golang/snappy v0.0.4 // indirect
	github.com/golangci/golangci-lint v1.38.0
	github.com/google/go-cmp v0.5.6
	github.com/google/uuid v1.3.0
	github.com/gorilla/mux v1.8.0
	github.com/gotestyourself/gotestyourself v2.2.0+incompatible // indirect
	github.com/hashicorp/errwrap v1.1.0 // indirect
	github.com/hashicorp/go-cleanhttp v0.5.2 // indirect
	github.com/hashicorp/go-multierror v1.1.1
	github.com/hashicorp/go-retryablehttp v0.7.0 // indirect
	github.com/hashicorp/vault/api v1.1.1
	github.com/jackc/pgx/v4 v4.12.0
	github.com/jgautheron/goconst v1.5.1 // indirect
	github.com/jingyugao/rowserrcheck v1.1.0 // indirect
	github.com/julz/importas v0.0.0-20210419104244-841f0c0fe66d // indirect
	github.com/kelseyhightower/run v0.0.17
	github.com/leodido/go-urn v1.2.1 // indirect
	github.com/lstoll/awskms v0.0.0-20210310122415-d1696e9c112b
	github.com/mattn/go-isatty v0.0.13 // indirect
	github.com/mgechev/revive v1.1.0 // indirect
	github.com/mikehelmick/go-chaff v0.5.0
	github.com/mitchellh/mapstructure v1.4.1
	github.com/nishanths/exhaustive v0.2.3 // indirect
	github.com/opencontainers/runc v1.0.1 // indirect
	github.com/ory/dockertest v3.3.5+incompatible
	github.com/pierrec/lz4 v2.6.1+incompatible // indirect
	github.com/polyfloyd/go-errorlint v0.0.0-20210510181950-ab96adb96fea // indirect
	github.com/prometheus/common v0.29.0 // indirect
	github.com/prometheus/procfs v0.7.0 // indirect
	github.com/prometheus/statsd_exporter v0.21.0 // indirect
	github.com/rakutentech/jwk-go v1.0.1
	github.com/ryancurrah/gomodguard v1.2.3 // indirect
	github.com/securego/gosec/v2 v2.8.1 // indirect
	github.com/sethvargo/go-envconfig v0.3.5
	github.com/sethvargo/go-gcpkms v0.1.0
	github.com/sethvargo/go-retry v0.1.0
	github.com/sethvargo/zapw v0.1.0
	github.com/spf13/cobra v1.2.1 // indirect
	github.com/tetafro/godot v1.4.8 // indirect
	github.com/timakin/bodyclose v0.0.0-20200424151742-cb6215831a94
	github.com/tommy-muehle/go-mnd/v2 v2.4.0 // indirect
	github.com/ugorji/go v1.2.6 // indirect
	github.com/uudashr/gocognit v1.0.5 // indirect
	go.opencensus.io v0.23.0
	go.uber.org/atomic v1.9.0 // indirect
	go.uber.org/multierr v1.7.0 // indirect
	go.uber.org/zap v1.18.1
	golang.org/x/crypto v0.0.0-20210711020723-a769d52b0f97 // indirect
	golang.org/x/net v0.0.0-20210716203947-853a461950ff // indirect
	golang.org/x/oauth2 v0.0.0-20210628180205-a41e5a781914
	golang.org/x/sync v0.0.0-20210220032951-036812b2e83c
	golang.org/x/time v0.0.0-20210611083556-38a9dc6acbc6
	golang.org/x/tools v0.1.5
	google.golang.org/api v0.50.0
	google.golang.org/genproto v0.0.0-20210719143636-1d5a45f8e492
	google.golang.org/grpc v1.39.0
	google.golang.org/grpc/cmd/protoc-gen-go-grpc v1.1.0
	google.golang.org/protobuf v1.27.1
	gopkg.in/square/go-jose.v2 v2.6.0 // indirect
	honnef.co/go/tools v0.2.1
	mvdan.cc/gofumpt v0.1.1 // indirect
)
