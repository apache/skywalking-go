// Licensed to Apache Software Foundation (ASF) under one or more contributor
// license agreements. See the NOTICE file distributed with
// this work for additional information regarding copyright
// ownership. Apache Software Foundation (ASF) licenses this file to you under
// the Apache License, Version 2.0 (the "License"); you may
// not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.

package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"

	_ "github.com/apache/skywalking-go"
)

var (
	uri           = "amqp://admin:123456@amqp-server:5672"
	exchange      = "sw-exchange"
	exchangeType  = "direct"
	queue         = "sw-queue"
	routingKey    = "sw-key"
	body          = "I love skywalking three thousand"
	consumerTag   = "sw-consumer"
	lifetime      = 3 * time.Second
	deliveryCount = 0

	WarnLog = log.New(os.Stderr, "[WARNING] ", log.LstdFlags|log.Lmsgprefix)
	ErrLog  = log.New(os.Stderr, "[ERROR] ", log.LstdFlags|log.Lmsgprefix)
	Log     = log.New(os.Stdout, "[INFO] ", log.LstdFlags|log.Lmsgprefix)
)

func main() {
	route := http.NewServeMux()
	route.HandleFunc("/execute", func(res http.ResponseWriter, req *http.Request) {
		producer()
	})
	route.HandleFunc("/consumer", func(res http.ResponseWriter, req *http.Request) {
		consumer()
	})
	route.HandleFunc("/health", func(res http.ResponseWriter, req *http.Request) {
		res.Write([]byte("ok"))
	})
	err := http.ListenAndServe(":8080", route)
	if err != nil {
		log.Fatalf("client start error: %v \n", err)
	}
}

func producer() {
	exitCh := make(chan struct{})
	confirmsCh := make(chan *amqp.DeferredConfirmation)
	confirmsDoneCh := make(chan struct{})
	publishOkCh := make(chan struct{}, 1)

	setupCloseHandler(exitCh)

	startConfirmHandler(publishOkCh, confirmsCh, confirmsDoneCh, exitCh)

	publish(context.Background(), publishOkCh, confirmsCh, confirmsDoneCh, exitCh)
}

func consumer() {
	c, err := NewConsumer(uri, exchange, exchangeType, queue, routingKey, consumerTag)
	if err != nil {
		ErrLog.Fatalf("%s", err)
	}
	SetupCloseHandler(c)

	Log.Printf("running for %s", lifetime)
	time.Sleep(lifetime)

	Log.Printf("shutting down")

	if err := c.Shutdown(); err != nil {
		ErrLog.Fatalf("error during shutdown: %s", err)
	}
}

type Consumer struct {
	conn    *amqp.Connection
	channel *amqp.Channel
	tag     string
	done    chan error
}

func SetupCloseHandler(consumer *Consumer) {
	c := make(chan os.Signal, 2)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		Log.Printf("Ctrl+C pressed in Terminal")
		if err := consumer.Shutdown(); err != nil {
			ErrLog.Fatalf("error during shutdown: %s", err)
		}
		os.Exit(0)
	}()
}

func NewConsumer(amqpURI, exchange, exchangeType, queueName, key, ctag string) (*Consumer, error) {
	c := &Consumer{
		conn:    nil,
		channel: nil,
		tag:     ctag,
		done:    make(chan error),
	}

	var err error

	config := amqp.Config{Properties: amqp.NewConnectionProperties()}
	config.Properties.SetClientConnectionName("sample-consumer")
	Log.Printf("dialing %q", amqpURI)
	c.conn, err = amqp.DialConfig(amqpURI, config)
	if err != nil {
		return nil, fmt.Errorf("dial: %s", err)
	}

	go func() {
		Log.Printf("closing: %s", <-c.conn.NotifyClose(make(chan *amqp.Error)))
	}()

	Log.Printf("got Connection, getting Channel")
	c.channel, err = c.conn.Channel()
	if err != nil {
		return nil, fmt.Errorf("channel: %s", err)
	}

	Log.Printf("got Channel, declaring Exchange (%q)", exchange)
	if err = c.channel.ExchangeDeclare(
		exchange,     // name of the exchange
		exchangeType, // type
		true,         // durable
		false,        // delete when complete
		false,        // internal
		false,        // noWait
		nil,          // arguments
	); err != nil {
		return nil, fmt.Errorf("exchange Declare: %s", err)
	}

	Log.Printf("declared Exchange, declaring Queue %q", queueName)
	queue, err := c.channel.QueueDeclare(
		queueName, // name of the queue
		true,      // durable
		false,     // delete when unused
		false,     // exclusive
		false,     // noWait
		nil,       // arguments
	)
	if err != nil {
		return nil, fmt.Errorf("queue Declare: %s", err)
	}

	Log.Printf("declared Queue (%q %d messages, %d consumers), binding to Exchange (key %q)",
		queue.Name, queue.Messages, queue.Consumers, key)

	if err = c.channel.QueueBind(
		queue.Name, // name of the queue
		key,        // bindingKey
		exchange,   // sourceExchange
		false,      // noWait
		nil,        // arguments
	); err != nil {
		return nil, fmt.Errorf("queue Bind: %s", err)
	}

	Log.Printf("Queue bound to Exchange, starting Consume (consumer tag %q)", c.tag)
	deliveries, err := c.channel.Consume(
		queue.Name, // name
		c.tag,      // consumerTag,
		true,       // autoAck
		false,      // exclusive
		false,      // noLocal
		false,      // noWait
		nil,        // arguments
	)
	if err != nil {
		return nil, fmt.Errorf("queue Consume: %s", err)
	}

	go handle(deliveries, c.done)

	return c, nil
}

func (c *Consumer) Shutdown() error {
	// will close() the deliveries channel
	if err := c.channel.Cancel(c.tag, true); err != nil {
		return fmt.Errorf("consumer cancel failed: %s", err)
	}

	if err := c.conn.Close(); err != nil {
		return fmt.Errorf("AMQP connection close error: %s", err)
	}

	defer Log.Printf("AMQP shutdown OK")

	// wait for handle() to exit
	return <-c.done
}

func handle(deliveries <-chan amqp.Delivery, done chan error) {
	cleanup := func() {
		Log.Printf("handle: deliveries channel closed")
		done <- nil
	}

	defer cleanup()

	for d := range deliveries {
		deliveryCount++
		Log.Printf(
			"got %dB delivery: [%v] %q",
			len(d.Body),
			d.DeliveryTag,
			d.Body,
		)
	}
}

func setupCloseHandler(exitCh chan struct{}) {
	c := make(chan os.Signal, 2)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		Log.Printf("close handler: Ctrl+C pressed in Terminal")
		close(exitCh)
	}()
}

func publish(ctx context.Context, publishOkCh <-chan struct{}, confirmsCh chan<- *amqp.DeferredConfirmation, confirmsDoneCh <-chan struct{}, exitCh chan struct{}) {
	config := amqp.Config{
		Vhost:      "/",
		Properties: amqp.NewConnectionProperties(),
	}
	config.Properties.SetClientConnectionName("producer-with-confirms")

	Log.Printf("producer: dialing %s", uri)
	conn, err := amqp.DialConfig(uri, config)
	if err != nil {
		ErrLog.Fatalf("producer: error in dial: %s", err)
	}
	defer conn.Close()

	Log.Println("producer: got Connection, getting Channel")
	channel, err := conn.Channel()
	if err != nil {
		ErrLog.Fatalf("error getting a channel: %s", err)
	}
	defer channel.Close()

	Log.Printf("producer: declaring exchange")
	if err := channel.ExchangeDeclare(
		exchange,     // name
		exchangeType, // type
		true,         // durable
		false,        // auto-delete
		false,        // internal
		false,        // noWait
		nil,          // arguments
	); err != nil {
		ErrLog.Fatalf("producer: Exchange Declare: %s", err)
	}

	Log.Printf("producer: declaring queue '%s'", queue)
	queue, err := channel.QueueDeclare(
		queue, // name of the queue
		true,  // durable
		false, // delete when unused
		false, // exclusive
		false, // noWait
		nil,   // arguments
	)
	if err == nil {
		Log.Printf("producer: declared queue (%q %d messages, %d consumers), binding to Exchange (key %q)",
			queue.Name, queue.Messages, queue.Consumers, routingKey)
	} else {
		ErrLog.Fatalf("producer: Queue Declare: %s", err)
	}

	Log.Printf("producer: declaring binding")
	if err := channel.QueueBind(queue.Name, routingKey, exchange, false, nil); err != nil {
		ErrLog.Fatalf("producer: Queue Bind: %s", err)
	}

	// Reliable publisher confirms require confirm.select support from the
	// connection.
	Log.Printf("producer: enabling publisher confirms.")
	if err := channel.Confirm(false); err != nil {
		ErrLog.Fatalf("producer: channel could not be put into confirm mode: %s", err)
	}

	for {
		canPublish := false
		Log.Println("producer: waiting on the OK to publish...")
		for {
			select {
			case <-confirmsDoneCh:
				Log.Println("producer: stopping, all confirms seen")
				return
			case <-publishOkCh:
				Log.Println("producer: got the OK to publish")
				canPublish = true
				break
			case <-time.After(time.Second):
				WarnLog.Println("producer: still waiting on the OK to publish...")
				continue
			}
			if canPublish {
				break
			}
		}

		Log.Printf("producer: publishing %dB body (%q)", len(body), body)
		dConfirmation, err := channel.PublishWithDeferredConfirmWithContext(
			ctx,
			exchange,
			routingKey,
			true,
			false,
			amqp.Publishing{
				Headers:         amqp.Table{},
				ContentType:     "text/plain",
				ContentEncoding: "",
				DeliveryMode:    amqp.Persistent,
				Priority:        0,
				AppId:           "sequential-producer",
				Body:            []byte(body),
			},
		)
		if err != nil {
			ErrLog.Fatalf("producer: error in publish: %s", err)
		}

		select {
		case <-confirmsDoneCh:
			Log.Println("producer: stopping, all confirms seen")
			return
		case confirmsCh <- dConfirmation:
			Log.Println("producer: delivered deferred confirm to handler")
			break
		}

		select {
		case <-confirmsDoneCh:
			Log.Println("producer: stopping, all confirms seen")
			return
		case <-time.After(time.Millisecond * 250):
			Log.Println("producer: initiating stop")
			close(exitCh)
			select {
			case <-confirmsDoneCh:
				Log.Println("producer: stopping, all confirms seen")
				return
			case <-time.After(time.Second * 10):
				WarnLog.Println("producer: may be stopping with outstanding confirmations")
				return
			}
		}
	}
}

func startConfirmHandler(publishOkCh chan<- struct{}, confirmsCh <-chan *amqp.DeferredConfirmation, confirmsDoneCh chan struct{}, exitCh <-chan struct{}) {
	go func() {
		confirms := make(map[uint64]*amqp.DeferredConfirmation)

		for {
			select {
			case <-exitCh:
				exitConfirmHandler(confirms, confirmsDoneCh)
				return
			default:
				break
			}

			outstandingConfirmationCount := len(confirms)

			if outstandingConfirmationCount <= 8 {
				select {
				case publishOkCh <- struct{}{}:
					Log.Println("confirm handler: sent OK to publish")
				case <-time.After(time.Second * 5):
					WarnLog.Println("confirm handler: timeout indicating OK to publish (this should never happen!)")
				}
			} else {
				WarnLog.Printf("confirm handler: waiting on %d outstanding confirmations, blocking publish", outstandingConfirmationCount)
			}

			select {
			case confirmation := <-confirmsCh:
				dtag := confirmation.DeliveryTag
				confirms[dtag] = confirmation
			case <-exitCh:
				exitConfirmHandler(confirms, confirmsDoneCh)
				return
			}

			checkConfirmations(confirms)
		}
	}()
}

func exitConfirmHandler(confirms map[uint64]*amqp.DeferredConfirmation, confirmsDoneCh chan struct{}) {
	Log.Println("confirm handler: exit requested")
	waitConfirmations(confirms)
	close(confirmsDoneCh)
	Log.Println("confirm handler: exiting")
}

func checkConfirmations(confirms map[uint64]*amqp.DeferredConfirmation) {
	Log.Printf("confirm handler: checking %d outstanding confirmations", len(confirms))
	for k, v := range confirms {
		if v.Acked() {
			Log.Printf("confirm handler: confirmed delivery with tag: %d", k)
			delete(confirms, k)
		}
	}
}

func waitConfirmations(confirms map[uint64]*amqp.DeferredConfirmation) {
	Log.Printf("confirm handler: waiting on %d outstanding confirmations", len(confirms))

	checkConfirmations(confirms)

	for k, v := range confirms {
		select {
		case <-v.Done():
			Log.Printf("confirm handler: confirmed delivery with tag: %d", k)
			delete(confirms, k)
		case <-time.After(time.Second):
			WarnLog.Printf("confirm handler: did not receive confirmation for tag %d", k)
		}
	}

	outstandingConfirmationCount := len(confirms)
	if outstandingConfirmationCount > 0 {
		ErrLog.Printf("confirm handler: exiting with %d outstanding confirmations", outstandingConfirmationCount)
	} else {
		Log.Println("confirm handler: done waiting on outstanding confirmations")
	}
}
