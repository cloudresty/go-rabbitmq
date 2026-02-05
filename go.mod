module github.com/cloudresty/go-rabbitmq

go 1.25.3

require github.com/rabbitmq/amqp091-go v1.10.0

require github.com/cloudresty/ulid v1.2.1

require github.com/cloudresty/go-env v1.0.1

require (
	github.com/rabbitmq/rabbitmq-stream-go-client v1.6.3
	go.opentelemetry.io/otel v1.40.0
	go.opentelemetry.io/otel/metric v1.40.0
	go.opentelemetry.io/otel/trace v1.40.0
	google.golang.org/protobuf v1.36.6
)

require (
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/golang/snappy v1.0.0 // indirect
	github.com/klauspost/compress v1.18.2 // indirect
	github.com/kr/pretty v0.3.1 // indirect
	github.com/pierrec/lz4 v2.6.1+incompatible // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/spaolacci/murmur3 v1.1.0 // indirect
)
