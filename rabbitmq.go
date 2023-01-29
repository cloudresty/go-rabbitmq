// RabbitMQ package for Go based on the AMQP library.
package rabbitmq

// Struct that defines the RabbitMQ AMQP connection settings.
type ConnectionSettings struct {
	Host     string
	Port     string
	User     string
	Password string
	Vhost    string
}

// Struct that defines the RabbitMQ AMQP queue settings.
type QueueSettings struct {
	Name       string
	RoutingKey string
	Durable    bool
	AutoDelete bool
	Exclusive  bool
	NoWait     bool
}

// Struct that defines the RabbitMQ AMQP exchange settings.
type ExchangeSettings struct {
	Name       string
	Type       string
	Durable    bool
	AutoDelete bool
	Internal   bool
	NoWait     bool
}

// Struct that defines the RabbitMQ AMQP binding settings.
type BindingSettings struct {
	Queue    QueueSettings
	Exchange ExchangeSettings
	Key      string
	NoWait   bool
}

// Struct that defines the RabbitMQ AMQP message settings.
type MessageSettings struct {
	ContentType string
	Body        []byte
}

// Struct that defines the RabbitMQ AMQP publisher settings.
type PublisherSettings struct {
	Exchange ExchangeSettings
}

// Struct that defines the RabbitMQ AMQP consumer settings.
type ConsumerSettings struct {
	Exchange   ExchangeSettings
	Queue      QueueSettings
	RoutingKey string
	NoWait     bool
}
