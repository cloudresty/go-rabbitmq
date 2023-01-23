// Subscriber is a simple example of a subscriber to a RabbitMQ queue.
package rabbitmq

import (
	amqp "github.com/rabbitmq/amqp091-go"
)

type Subscriber struct {
	// Connection settings.
	Connection ConnectionSettings
}

// NewSubscriber creates a new Subscriber.
func NewSubscriber(connection ConnectionSettings) *Subscriber {
	return &Subscriber{
		Connection: connection,
	}
}

// Subscribe subscribes to a RabbitMQ queue.
func (s *Subscriber) Subscribe(settings SubscriberSettings, callback func(message MessageSettings)) error {

	// Connect to RabbitMQ.
	conn, err := amqp.Dial("amqp://" + s.Connection.User + ":" + s.Connection.Password + "@" + s.Connection.Host + ":" + s.Connection.Port + "/" + s.Connection.Vhost)
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

	// Declare the queue.
	q, err := ch.QueueDeclare(
		settings.Queue.Name,    // name
		settings.Queue.Durable, // durable
		settings.Queue.AutoDelete,
		settings.Queue.Exclusive,
		settings.Queue.NoWait,
		nil, // arguments
	)
	if err != nil {
		return err
	}

	// Bind the queue to the exchange.
	err = ch.QueueBind(
		q.Name,                    // queue name
		settings.Queue.RoutingKey, // routing key
		settings.Exchange.Name,    // exchange
		settings.Queue.NoWait,
		nil, // arguments
	)
	if err != nil {
		return err
	}

	// Consume messages.
	msgs, err := ch.Consume(
		q.Name, // queue
		"",     // consumer
		true,   // auto-ack
		false,  // exclusive
		false,  // no-local
		false,  // no-wait
		nil,    // args
	)
	if err != nil {
		return err
	}

	// Process messages.
	forever := make(chan bool)
	go func() {
		for d := range msgs {
			callback(MessageSettings{
				Body: d.Body,
			})
		}
	}()
	<-forever

	return nil
}
