// RabbitMQ package for Go based on the AMQP library.
package rabbitmq

import amqp "github.com/rabbitmq/amqp091-go"

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
	Type       string
	Args       amqp.Table
}

// QoSSettings defines the RabbitMQ AMQP QoS settings.
type QoSSettings struct {
	PrefetchCount int
	PrefetchSize  int
	Global        bool
}

// Struct that defines the RabbitMQ AMQP exchange settings.
type ExchangeSettings struct {
	Name       string
	Type       string
	Durable    bool
	AutoDelete bool
	Internal   bool
	NoWait     bool
	Args       amqp.Table
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
	MessageId   string
	ContentType string
	Body        []byte
}

// Struct that defines the RabbitMQ AMQP publisher settings.
type PublisherSettings struct {
	Exchange ExchangeSettings
	Queue    QueueSettings
}

// Struct that defines the RabbitMQ AMQP consumer settings.
type ConsumerSettings struct {
	Exchange   ExchangeSettings
	Queue      QueueSettings
	QoS        QoSSettings
	RoutingKey string
	NoWait     bool
	AutoAck    bool
}

// Struct that defines the RabbitMQ Channel settings.
type ChannelSettings struct {
	Channel *amqp.Channel
}
