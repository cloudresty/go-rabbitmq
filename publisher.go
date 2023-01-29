// Publisher connects to RabbitMQ and publishes messages to a queue
// using the AMQP protocol.
package rabbitmq

import (
	"context"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

// Publisher connects to RabbitMQ and publishes messages to a queue
// using the AMQP protocol.
type Publisher struct {
	// Connection settings.
	Connection ConnectionSettings
}

// NewPublisher creates a new Publisher.
func NewPublisher(connection ConnectionSettings) *Publisher {
	return &Publisher{
		Connection: connection,
	}
}

// Publish publishes a message to RabbitMQ.
func (p *Publisher) Publish(settings PublisherSettings, message MessageSettings) error {

	// Connect to RabbitMQ.
	conn, err := amqp.Dial("amqp://" + p.Connection.User + ":" + p.Connection.Password + "@" + p.Connection.Host + ":" + p.Connection.Port + "/" + p.Connection.Vhost)
	if err != nil {
		return err
	}
	defer conn.Close()

	// Open a channel.
	ch, err := conn.Channel()
	if err != nil {
		return err
	}
	defer ch.Close()

	// Declare the exchange.
	err = ch.ExchangeDeclare(
		settings.Exchange.Name, // name
		settings.Exchange.Type, // type
		settings.Exchange.Durable,
		settings.Exchange.AutoDelete,
		settings.Exchange.Internal,
		settings.Exchange.NoWait,
		nil, // arguments
	)
	if err != nil {

		return err
	}

	// Create a context with a timeout.
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Publish the message.
	err = ch.PublishWithContext(ctx,

		settings.Exchange.Name, // exchange
		"",                     // routing key
		false,                  // mandatory
		false,                  // immediate

		amqp.Publishing{
			MessageId:   message.MessageId,
			ContentType: message.ContentType,
			Body:        message.Body,
		},
	)

	if err != nil {
		return err
	}

	return nil

}
