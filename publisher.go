package rabbitmq

import (
	"log"

	amqp "github.com/rabbitmq/amqp091-go"
)

type Publisher struct {
	connection *amqp.Connection
}

func (e *Publisher) setup() error {
	channel, err := e.connection.Channel()
	if err != nil {
		panic(err)
	}

	defer channel.Close()
	return declareExchange(channel)
}

// Publish a specified message to the AMQP exchange
func (e *Publisher) Publish(event string, severity string) error {
	channel, err := e.connection.Channel()
	if err != nil {
		return err
	}

	defer channel.Close()

	err = channel.Publish(
		getExchangeName(),
		severity,
		false,
		false,
		amqp.Publishing{
			ContentType: "text/plain",
			Body:        []byte(event),
		},
	)
	log.Printf("Sending message: %s -> %s", event, getExchangeName())
	return nil
}

// NewPublisherEvent returns a new event.Emitter object
// ensuring that the object is initialized, without error
func NewPublisherEvent(conn *amqp.Connection) (Publisher, error) {
	publisher := Publisher{
		connection: conn,
	}

	err := publisher.setup()
	if err != nil {
		return Publisher{}, err
	}

	return publisher, nil

}
