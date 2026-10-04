module github.com/devilmonastery/hivemind

go 1.26.4

require (
	github.com/bwmarrin/snowflake v0.3.0
	github.com/golang-jwt/jwt/v5 v5.3.1
	github.com/golang-migrate/migrate/v4 v4.20.1
	github.com/gorilla/mux v1.8.1
	github.com/gorilla/sessions v1.4.0
	github.com/jmoiron/sqlx v1.4.0
	github.com/lib/pq v1.12.3
	github.com/microcosm-cc/bluemonday v1.0.27
	github.com/russross/blackfriday/v2 v2.1.0
	github.com/spf13/cobra v1.10.2
	golang.org/x/crypto v0.57.0
	golang.org/x/oauth2 v0.37.0
	google.golang.org/grpc v1.84.0
	google.golang.org/protobuf v1.36.12
	gopkg.in/yaml.v2 v2.4.0
	gopkg.in/yaml.v3 v3.0.1
)

require (
	github.com/docker/docker v28.4.0+incompatible // indirect
	github.com/gorilla/websocket v1.5.4-0.20250319132907-e064f32e3674 // indirect
	github.com/gosimple/unidecode v1.0.1 // indirect
)

require (
	github.com/aymerick/douceur v0.2.0 // indirect
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/bwmarrin/discordgo v0.29.1-0.20251229161010-9f6aa8159fc6
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/devilmonastery/hivemind/api v0.3.0
	github.com/gorilla/css v1.0.1 // indirect
	github.com/gorilla/securecookie v1.1.2 // indirect
	github.com/gosimple/slug v1.15.0
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/munnerz/goautoneg v0.0.0-20191010083416-a7dc8b61c822 // indirect
	github.com/prometheus/client_golang v1.24.1
	github.com/prometheus/client_model v0.6.2 // indirect
	github.com/prometheus/common v0.70.1 // indirect
	github.com/prometheus/procfs v0.21.1 // indirect
	github.com/spf13/pflag v1.0.9 // indirect
	go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp v0.63.0 // indirect
	golang.org/x/net v0.58.0 // indirect
	golang.org/x/sys v0.47.0 // indirect
	golang.org/x/text v0.41.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20260526163538-3dc84a4a5aaa // indirect
)

replace github.com/devilmonastery/hivemind/api => ./api
