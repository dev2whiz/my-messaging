package handler

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/my-messaging/gateway/internal/broker"
	"github.com/my-messaging/gateway/internal/middleware"
	"github.com/my-messaging/gateway/internal/model"
)

var upgrader = websocket.Upgrader{
	HandshakeTimeout: 10 * time.Second,
	ReadBufferSize:   1024,
	WriteBufferSize:  4096,
	CheckOrigin:      func(r *http.Request) bool { return true }, // tighten in Stage 5
}

type WsHandler struct {
	broker *broker.Broker
}

func NewWsHandler(b *broker.Broker) *WsHandler {
	return &WsHandler{broker: b}
}

// GET /ws?token=<jwt>
// Upgraded to WebSocket; bridges the user's RabbitMQ queue to the connection.
func (h *WsHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	caller := middleware.GetUser(r.Context())
	if caller == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("ws: upgrade error: %v", err)
		return
	}
	defer conn.Close()

	log.Printf("ws: user %s connected", caller.UserID)

	deliveries, err := h.broker.Consume(caller.UserID)
	if err != nil {
		log.Printf("ws: broker consume error for user %s: %v", caller.UserID, err)
		conn.WriteJSON(model.WsFrame{Type: "error", Payload: model.Message{Body: "broker error"}})
		return
	}

	// Goroutine: forward RabbitMQ deliveries → WebSocket
	done := make(chan struct{})
	go func() {
		defer close(done)
		for d := range deliveries {
			var frame model.WsFrame
			if err := json.Unmarshal(d.Body, &frame); err != nil {
				d.Nack(false, false)
				continue
			}
			if err := conn.WriteJSON(frame); err != nil {
				log.Printf("ws: write error for user %s: %v", caller.UserID, err)
				d.Nack(false, true) // requeue
				return
			}
			d.Ack(false)
		}
	}()

	// Read loop: handle client pings / connection close
	conn.SetReadLimit(512)
	conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	conn.SetPongHandler(func(string) error {
		conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	pingDone := make(chan struct{})
	go func() {
		defer close(pingDone)
		for {
			select {
			case <-ticker.C:
				conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
				if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
					return
				}
			case <-done:
				return
			}
		}
	}()

	for {
		_, _, err := conn.ReadMessage()
		if err != nil {
			break
		}
	}

	log.Printf("ws: user %s disconnected", caller.UserID)
	<-pingDone
}

// Drain any pending amqp.Delivery channel safely.
func drainDeliveries(ch <-chan amqp.Delivery) {
	for range ch {
	}
}
