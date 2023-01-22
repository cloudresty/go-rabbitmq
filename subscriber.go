package rabbitmq

import (
	"log"

	amqp "github.com/rabbitmq/amqp091-go"
)

// Subscriber for receiving AMPQ events
type Subscriber struct {
	conn      *amqp.Connection
	queueName string
}

func (subscriber *Subscriber) setup() error {
	channel, err := subscriber.conn.Channel()
	if err != nil {
		return err
	}
	return declareExchange(channel)
}

// NewSubscriber returns a new Subscriber
func NewConsumer(conn *amqp.Connection) (Subscriber, error) {
	subscriber := Subscriber{
		conn: conn,
	}
	err := subscriber.setup()
	if err != nil {
		return Subscriber{}, err
	}

	return subscriber, nil
}

// Listen will listen for all new Queue publications
// and print them to the console.
func (subscriber *Subscriber) Listen(topics []string) error {
	ch, err := subscriber.conn.Channel()
	if err != nil {
		return err
	}
	defer ch.Close()

	q, err := declareRandomQueue(ch)
	if err != nil {
		return err
	}

	for _, s := range topics {
		err = ch.QueueBind(
			q.Name,
			s,
			getExchangeName(),
			false,
			nil,
		)

		if err != nil {
			return err
		}
	}

	msgs, err := ch.Consume(q.Name, "", true, false, false, false, nil)
	if err != nil {
		return err
	}

	forever := make(chan bool)
	go func() {
		for d := range msgs {
			log.Printf("Received a message: %s", d.Body)
		}
	}()

	log.Printf("[*] Waiting for message [Exchange, Queue][%s, %s]. To exit press CTRL+C", getExchangeName(), q.Name)
	<-forever
	return nil
}
