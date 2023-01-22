# GoRabbitMQ Package

This is a lightweight Go package for RabbitMQ.

What you can find on this page:

- [GoRabbitMQ Setup](#gorabbitmq-setup)
  - [Package version](#package-version)
  - [Package install](#package-install)
  - [Package import](#package-import)
- [Publisher Example](#publisher-example)
- [Subscriber Example](#subscriber-example)

&nbsp;

## GoRabbitMQ Setup

&nbsp;

### Package version

Available versions...

```shell
go list -m -versions github.com/cloudresty/gorabbitmq
```

&nbsp;

### Package install

Package installation...

```shell
go get github.com/cloudresty/gorabbitmq@v1.0.0
```

&nbsp;

### Package import

Package import...

```go
package main

import rabbitmq github.com/cloudresty/gorabbitmq

// ...
```

&nbsp;

## Publisher example

```go
package main

import (
    "fmt"
    "os"

    rabbitmq "github.com/cloudresty/gorabbitmq"
    amqp "github.com/rabbitmq/amqp091-go"
)

func main() {

    conn, err := amqp.Dial("amqp://guest:guest@localhost:5672")
    if err != nil {
        panic(err)
    }

    publisher, err := rabbitmq.NewEventEmitter(conn)
    if err != nil {
        panic(err)
    }

    for i := 1; i < 10; i++ {
        emitter.Push(fmt.Sprintf("[%d] - %s", i, os.Args[1]), os.Args[1])
    }
}
```

&nbsp;

## Subscriber example

```go
package main

import (
    "os"

    rabbitmq "github.com/cloudresty/gorabbitmq"
    amqp     "github.com/rabbitmq/amqp091-go"
)

func main() {

    connection, err := amqp.Dial("amqp://guest:guest@localhost:5672")
    if err != nil {
        panic(err)
    }
    defer connection.Close()

    subscriber, err := rabbitmq.NewSubscriber(connection)
    if err != nil {
        panic(err)
    }
    subscriber.Listen(os.Args[1:])
}
```

&nbsp;

---
Copyright &copy; [Cloudresty](https://cloudresty.com)