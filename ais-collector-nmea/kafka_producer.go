package main

import (
	"log/slog"

	"github.com/confluentinc/confluent-kafka-go/kafka"
)

func NewKafkaProducer(cfg Config) (*kafka.Producer, error) {
	producer, err := kafka.NewProducer(&kafka.ConfigMap{
		"bootstrap.servers": cfg.KafkaBootstrapServers,
	})
	if err != nil {
		return nil, err
	}

	go func() {
		for e := range producer.Events() {
			if msg, ok := e.(*kafka.Message); ok && msg.TopicPartition.Error != nil {
				slog.Error("kafka delivery failed", "error", msg.TopicPartition.Error)
			}
		}
	}()

	return producer, nil
}

func Produce(producer *kafka.Producer, cfg Config, mmsi string, sentence string) {
	var key []byte
	if mmsi != "" {
		key = []byte(mmsi)
	}

	topic := cfg.KafkaTopic
	err := producer.Produce(&kafka.Message{
		TopicPartition: kafka.TopicPartition{Topic: &topic, Partition: kafka.PartitionAny},
		Key:            key,
		Value:          []byte(sentence),
	}, nil)
	if err != nil {
		slog.Error("failed to enqueue message for production", "error", err)
	}
}
