module github.com/cloudresty/go-rabbitmq

go 1.26.0

// v1.11.3 was published missing two commits of the reconnect fix: a narrow
// window where a retried publish could lose its callback silently, a path that
// could fire two callbacks for one message, and unbounded automatic retries
// after a nack. Use v1.12.0 or later.
retract v1.11.3

require github.com/rabbitmq/amqp091-go v1.15.0

require github.com/cloudresty/ulid v1.2.1

require github.com/cloudresty/go-env v1.0.1

require (
	github.com/rabbitmq/rabbitmq-stream-go-client v1.8.3
	go.opentelemetry.io/otel v1.47.0
	go.opentelemetry.io/otel/metric v1.47.0
	go.opentelemetry.io/otel/trace v1.47.0
	google.golang.org/protobuf v1.36.12
)

require (
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/go-logr/logr v1.4.4 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/golang/snappy v1.0.0 // indirect
	github.com/klauspost/compress v1.20.1 // indirect
	github.com/kr/text v0.2.0 // indirect
	github.com/pierrec/lz4 v2.6.1+incompatible // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/rogpeppe/go-internal v1.16.0 // indirect
	github.com/spaolacci/murmur3 v1.1.0 // indirect
	go.opentelemetry.io/auto/sdk v1.2.1 // indirect
	go.opentelemetry.io/otel/log v1.47.0 // indirect
)
