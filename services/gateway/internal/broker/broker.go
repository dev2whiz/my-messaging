package broker

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	amqp "github.com/rabbitmq/amqp091-go"
)

const (
	ExchangeName = "messaging"       // topic exchange
	ExchangeType = "topic"
)

type Broker struct {
	conn *amqp.Connection
	ch   *amqp.Channel
}

func New(url string) (*Broker, error) {
	conn, err := amqp.Dial(url)
	if err != nil {
		return nil, fmt.Errorf("broker: dial: %w", err)
	}
	ch, err := conn.Channel()
	if err != nil {
		return nil, fmt.Errorf("broker: channel: %w", err)
	}
	// Declare the durable topic exchange once at startup.
	if err := ch.ExchangeDeclare(ExchangeName, ExchangeType, true, false, false, false, nil); err != nil {
		return nil, fmt.Errorf("broker: exchange declare: %w", err)
	}
	return &Broker{conn: conn, ch: ch}, nil
}

// DeclareUserQueue ensures a durable queue exists for the user and binds it on the exchange.
func (b *Broker) DeclareUserQueue(userID string) error {
	qName := userQueueName(userID)
	q, err := b.ch.QueueDeclare(qName, true, false, false, false, nil)
	if err != nil {
		return fmt.Errorf("broker: queue declare %s: %w", qName, err)
	}
	return b.ch.QueueBind(q.Name, routingKey(userID), ExchangeName, false, nil)
}

// Publish sends a JSON-encoded payload to a user's queue via the exchange.
func (b *Broker) Publish(ctx context.Context, recipientID string, payload any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("broker: marshal: %w", err)
	}
	return b.ch.PublishWithContext(ctx, ExchangeName, routingKey(recipientID), false, false,
		amqp.Publishing{
			ContentType:  "application/json",
			DeliveryMode: amqp.Persistent,
			Body:         body,
		})
}

// Consume starts consuming messages from a user's queue, delivering to the returned channel.
func (b *Broker) Consume(userID string) (<-chan amqp.Delivery, error) {
	// New channel per consumer to avoid shared prefetch state.
	ch, err := b.conn.Channel()
	if err != nil {
		return nil, fmt.Errorf("broker: consumer channel: %w", err)
	}
	if err := ch.Qos(10, 0, false); err != nil {
		return nil, err
	}
	deliveries, err := ch.Consume(userQueueName(userID), "", false, false, false, false, nil)
	if err != nil {
		return nil, fmt.Errorf("broker: consume %s: %w", userID, err)
	}
	log.Printf("broker: consuming queue user.%s", userID)
	return deliveries, nil
}

func (b *Broker) Close() {
	b.ch.Close()
	b.conn.Close()
}

func userQueueName(userID string) string { return "user." + userID }
func routingKey(userID string) string    { return "user." + userID }
