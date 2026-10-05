package main

import (
	"context"
	"log"
	"os"
	"strings"
	"time"

	"github.com/sartim/storemesh-inventory-service/internal/repository"
	"github.com/segmentio/kafka-go"
)

func main() {
	databaseURL, brokers := os.Getenv("DATABASE_URL"), os.Getenv("KAFKA_BROKERS")
	if databaseURL == "" || brokers == "" {
		log.Fatal("DATABASE_URL and KAFKA_BROKERS are required")
	}
	store, err := repository.Open(context.Background(), databaseURL)
	if err != nil {
		log.Fatal(err)
	}
	defer store.Close()
	writer := &kafka.Writer{Addr: kafka.TCP(strings.Split(brokers, ",")...), Balancer: &kafka.Hash{}, BatchTimeout: 100 * time.Millisecond}
	defer writer.Close()
	workerID := os.Getenv("OUTBOX_WORKER_ID")
	if workerID == "" {
		workerID, err = os.Hostname()
		if err != nil {
			log.Fatal(err)
		}
	}
	ctx := context.Background()
	for {
		events, err := store.ClaimPendingOutbox(ctx, 100, workerID, 30*time.Second)
		if err != nil {
			log.Fatal(err)
		}
		for _, event := range events {
			if err := writer.WriteMessages(ctx, kafka.Message{Topic: "storemesh.inventory.events", Key: []byte(event.AggregateID), Value: event.Payload, Headers: []kafka.Header{{Key: "event-type", Value: []byte(event.EventType)}, {Key: "event-id", Value: []byte(event.ID)}}}); err != nil {
				log.Printf("publish %s: %v", event.ID, err)
				continue
			}
			if err := store.MarkOutboxPublished(ctx, event.ID, workerID, time.Now()); err != nil {
				log.Printf("mark %s: %v", event.ID, err)
			}
		}
		time.Sleep(time.Second)
	}
}
