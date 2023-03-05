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
// This method is not recommended for bulk publishing.
func (p *Publisher) Publish(publisherSettings PublisherSettings, message MessageSettings) error {

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
		publisherSettings.Exchange.Name,
		publisherSettings.Exchange.Type,
		publisherSettings.Exchange.Durable,
		publisherSettings.Exchange.AutoDelete,
		publisherSettings.Exchange.Internal,
		publisherSettings.Exchange.NoWait,
		publisherSettings.Exchange.Args,
	)
	if err != nil {

		return err
	}

	// Declare the queue.
	_, err = ch.QueueDeclare(
		publisherSettings.Queue.Name,
		publisherSettings.Queue.Durable,
		publisherSettings.Queue.AutoDelete,
		publisherSettings.Queue.Exclusive,
		publisherSettings.Queue.NoWait,
		publisherSettings.Queue.Args,
	)

	if err != nil {
		return err
	}

	// Bind the queue to the exchange.
	err = ch.QueueBind(
		publisherSettings.Queue.Name,
		publisherSettings.Queue.RoutingKey,
		publisherSettings.Exchange.Name,
		publisherSettings.Queue.NoWait,
		publisherSettings.Queue.Args,
	)

	if err != nil {
		return err
	}

	// Create a context with a timeout.
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Publish the message.
	err = ch.PublishWithContext(ctx,

		publisherSettings.Exchange.Name, // exchange
		"",                              // routing key
		false,                           // mandatory
		false,                           // immediate

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

// Publish with channel publishes one or more messages to RabbitMQ.
// This method is recommended for bulk publishing.
func (p *Publisher) PublishWithChannel(publisherSettings PublisherSettings, messages []MessageSettings) error {

	config := amqp.Config{
		Properties: amqp.Table{
			"connection_name": "bulk-publisher",
		},
	}

	// Connect to RabbitMQ.
	conn, errDialConfig := amqp.DialConfig("amqp://"+p.Connection.User+":"+p.Connection.Password+"@"+p.Connection.Host+":"+p.Connection.Port+"/"+p.Connection.Vhost, config)
	if errDialConfig != nil {
		return errDialConfig
	}
	defer conn.Close()

	// Open a channel.
	ch, errChannel := conn.Channel()
	if errChannel != nil {
		return errChannel
	}
	defer ch.Close()

	// Declare the exchange.
	errExchangeDeclare := ch.ExchangeDeclare(
		publisherSettings.Exchange.Name,
		publisherSettings.Exchange.Type,
		publisherSettings.Exchange.Durable,
		publisherSettings.Exchange.AutoDelete,
		publisherSettings.Exchange.Internal,
		publisherSettings.Exchange.NoWait,
		publisherSettings.Exchange.Args,
	)

	if errExchangeDeclare != nil {
		return errExchangeDeclare
	}

	// Declare the queue.
	_, errQueueDeclare := ch.QueueDeclare(
		publisherSettings.Queue.Name,
		publisherSettings.Queue.Durable,
		publisherSettings.Queue.AutoDelete,
		publisherSettings.Queue.Exclusive,
		publisherSettings.Queue.NoWait,
		publisherSettings.Queue.Args,
	)

	if errQueueDeclare != nil {
		return errQueueDeclare
	}

	// Bind the queue to the exchange.
	errQueueBind := ch.QueueBind(
		publisherSettings.Queue.Name,
		publisherSettings.Queue.RoutingKey,
		publisherSettings.Exchange.Name,
		publisherSettings.Queue.NoWait,
		publisherSettings.Queue.Args,
	)

	if errQueueBind != nil {
		return errQueueBind
	}

	// Create a context with a timeout.
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	for _, message := range messages {

		// Publish the message.
		errPublishWithContext := ch.PublishWithContext(ctx, publisherSettings.Exchange.Name, "", false, false, amqp.Publishing{
			MessageId:   message.MessageId,
			ContentType: message.ContentType,
			Body:        message.Body,
		})

		if errPublishWithContext != nil {
			return errPublishWithContext
		}

	}

	return nil

}
