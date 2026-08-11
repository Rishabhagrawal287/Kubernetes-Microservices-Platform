package main

import (
	"encoding/json"
	"log"
	"os"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/redis/go-redis/v9"
)

type orderCreatedEvent struct {
	Event     string `json:"event"`
	OrderID   int    `json:"order_id"`
	ProductID string `json:"product_id"`
	Quantity  int    `json:"quantity"`
}

// startOrderEventsConsumer connects to RabbitMQ and decrements stock whenever
// an order.created event arrives. It retries indefinitely with a fixed
// backoff on any connection failure or unexpected channel closure — a broker
// that's still booting, or that briefly restarts, should not permanently
// silence this consumer the way a single connect-and-give-up attempt would.
func startOrderEventsConsumer(rdb *redis.Client) {
	for {
		if err := consumeOnce(rdb); err != nil {
			log.Printf("product-service: consumer error, retrying in 5s: %v", err)
		} else {
			log.Println("product-service: consumer channel closed, retrying in 5s")
		}
		time.Sleep(5 * time.Second)
	}
}

// consumeOnce runs a single connect -> declare -> bind -> consume cycle.
// It returns (nil or an error) once the connection/channel drops, so the
// caller's loop can retry from scratch.
func consumeOnce(rdb *redis.Client) error {
	url := os.Getenv("RABBITMQ_URL")
	if url == "" {
		url = "amqp://guest:guest@localhost:5672/"
	}

	conn, err := amqp.Dial(url)
	if err != nil {
		return err
	}
	defer conn.Close()

	ch, err := conn.Channel()
	if err != nil {
		return err
	}
	defer ch.Close()

	if err := ch.ExchangeDeclare("order_events", "fanout", true, false, false, false, nil); err != nil {
		return err
	}

	q, err := ch.QueueDeclare("product_service.order_events", true, false, false, false, nil)
	if err != nil {
		return err
	}

	if err := ch.QueueBind(q.Name, "", "order_events", false, nil); err != nil {
		return err
	}

	msgs, err := ch.Consume(q.Name, "", true, false, false, false, nil)
	if err != nil {
		return err
	}

	log.Println("product-service: listening for order.created events")

	for msg := range msgs {
		var event orderCreatedEvent
		if err := json.Unmarshal(msg.Body, &event); err != nil {
			log.Printf("product-service: bad event payload: %v", err)
			continue
		}
		if err := decrementStock(rdb, event.ProductID, event.Quantity); err != nil {
			log.Printf("product-service: failed to decrement stock for %s: %v", event.ProductID, err)
			continue
		}
		inventoryDecrementsTotal.Inc()
		log.Printf("product-service: decremented stock for %s by %d (order %d)", event.ProductID, event.Quantity, event.OrderID)
	}

	// msgs channel closed: connection or channel dropped. Return nil so the
	// caller logs the "closed, retrying" message rather than a raw error.
	return nil
}
