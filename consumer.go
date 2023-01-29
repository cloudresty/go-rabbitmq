// Consumer is a simple example of a consumer to a RabbitMQ queue.
package rabbitmq

import (
	"fmt"

	amqp "github.com/rabbitmq/amqp091-go"
)

type Consumer struct {

	// Connection settings.
	Connection ConnectionSettings
}

// NewConsumer creates a new Consumer.
func NewConsumer(connection ConnectionSettings) *Consumer {

	return &Consumer{
		Connection: connection,
	}

}

// Subscribe subscribes to a RabbitMQ queue.
func (s *Consumer) Consume(settings ConsumerSettings, callback func(message MessageSettings), connectionName string) error {

	config := amqp.Config{
		Properties: amqp.Table{
			"connection_name": connectionName,
		},
	}

	// Connect to RabbitMQ.
	// connection, err := amqp.Dial("amqp://" + s.Connection.User + ":" + s.Connection.Password + "@" + s.Connection.Host + ":" + s.Connection.Port + "/" + s.Connection.Vhost)
	connection, err := amqp.DialConfig("amqp://"+s.Connection.User+":"+s.Connection.Password+"@"+s.Connection.Host+":"+s.Connection.Port+"/"+s.Connection.Vhost, config)
	if err != nil {
		return err
	}
	defer connection.Close()

	// Open a channel.
	channel, err := connection.Channel()
	if err != nil {
		return err
	}
	defer channel.Close()

	// Declare the exchange.
	err = channel.ExchangeDeclare(
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
	queue, err := channel.QueueDeclare(
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
	err = channel.QueueBind(
		queue.Name,                // queue name
		settings.Queue.RoutingKey, // routing key
		settings.Exchange.Name,    // exchange
		settings.Queue.NoWait,
		nil, // arguments
	)
	if err != nil {
		return err
	}

	// Consume messages.
	messages, err := channel.Consume(

		queue.Name,     // queue
		connectionName, // consumer
		false,          // auto-ack
		true,           // exclusive
		false,          // no-local
		false,          // no-wait
		nil,            // args

	)

	if err != nil {
		return err
	}

	// Process messages.
	forever := make(chan bool)

	go func() {

		for message := range messages {

			callback(MessageSettings{
				MessageId: message.MessageId,
				Body:      message.Body,
			})

			// Acknowledge the message.
			if err = message.Ack(false); err != nil {
				fmt.Printf("Unable to acknowledge the message '"+message.MessageId+"', err: %s", err.Error())
			}

		}

	}()

	<-forever

	return nil

}
