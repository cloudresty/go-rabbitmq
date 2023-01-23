# GoRabbitMQ Package

GoRabbitMQ is a very simple Go package for RabbitMQ.

What you can find on this page:

- [GoRabbitMQ Setup](#gorabbitmq-setup)
  - [Package version](#package-version)
  - [Package install](#package-install)
  - [Package import](#package-import)
- [Examples](#examples)
  - [Publisher Example](#publisher-example)
  - [Subscriber Example](#subscriber-example)

&nbsp;

## GoRabbitMQ Setup

&nbsp;

### Package version

You can check for the latest available version of GoRabbitMQ package by visiting the [tags page](https://github.com/cloudresty/gorabbitmq/tags) through your web browser or through the CLI like shown in the example below.

```shell
go list -m -versions github.com/cloudresty/gorabbitmq
```

&nbsp;

### Package install

The package can be installed via `go get` command.

```shell
go get github.com/cloudresty/gorabbitmq@v0.0.1
```

&nbsp;

### Package import

Like any other Go package this can be easily imported like shown in this example.

```go
package main

import rabbitmq github.com/cloudresty/gorabbitmq

// ...
```

&nbsp;

## Examples

Below you'll find two basic examples that demonstrates how to use GoRabbitMQ package, both examples covering the basic functionality for a `Publisher` and also for a `Subscriber`.

&nbsp;

### Publisher example

```go
package main

import (
    "encoding/json"
    "log"

    rabbitmq "github.com/cloudresty/gorabbitmq"
)

// Message structure
type Message struct {
    Name string
    Body string
    Time int64
}

// Main function
func main() {

    // Message content
    message := Message{"Cloudresty", "Hello", 1294706395881547000}

    // JSON Encode
    jsonMessage, err := json.Marshal(message)
    if err != nil {
        log.Println(err.Error())
    }

    // Publish message
    err = publishMessage("cloudresty", jsonMessage)
    if err != nil {
        log.Println(err.Error())
    }

}

// Publish a message to RabbitMQ
func publishMessage(exchange string, message []byte) error {

    // Create a new Publisher
    publisher := rabbitmq.NewPublisher(rabbitmq.ConnectionSettings{
        Host:     "localhost",
        Port:     "5672",
        User:     "guest",
        Password: "guest",
        Vhost:    "/",
    })

    // Publish a message
    err := publisher.Publish(rabbitmq.PublisherSettings{
        Exchange: rabbitmq.ExchangeSettings{
            Name:       exchange,
            Type:       "direct",
            Durable:    true,
            AutoDelete: false,
            Internal:   false,
            NoWait:     false,
        },
    }, rabbitmq.MessageSettings{
        ContentType: "text/plain",
        Body:        message,
    })
    if err != nil {
        return err
    }

    return nil

}
```

&nbsp;

### Subscriber example

```go
package main

import (
    log

    rabbitmq "github.com/cloudresty/gorabbitmq"
)

// Main function
func main(){

    err := rabbitMQSubscribe("cloudresty", "cloudresty")
    if err != nil {
        log.Println(err.Error())
    }

}

// RabbitMQ Subscribe function
func rabbitMQSubscribe(exchange, queue string) error {

    // Create a new Subscriber
    subscriber := rabbitmq.NewSubscriber(rabbitmq.ConnectionSettings{
        Host:     "localhost",
        Port:     "5672",
        User:     "guest",
        Password: "guest",
        Vhost:    "/",
    })

    err := subscriber.Subscribe(rabbitmq.SubscriberSettings{
        Exchange: rabbitmq.ExchangeSettings{
            Name:       exchange,
            Type:       "direct",
            Durable:    true,
            AutoDelete: false,
            Internal:   false,
            NoWait:     false,
        },
        Queue: rabbitmq.QueueSettings{
            Name:       queue,
            RoutingKey: queue,
            Durable:    true,
            AutoDelete: false,
            Exclusive:  false,
            NoWait:     false,
        },
        RoutingKey: queue,
        NoWait:     false,
    }, func(message rabbitmq.MessageSettings) {

        // Handle message
        log.Println("Message Received: "+string(message.Body))
    })

    if err != nil {
        return err
    }

}
```

&nbsp;

---
Copyright &copy; [Cloudresty](https://cloudresty.com)