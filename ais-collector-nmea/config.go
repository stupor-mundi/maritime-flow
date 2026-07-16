package main

import (
	"fmt"
	"os"
)

type Config struct {
	UDPPort               string
	KafkaBootstrapServers string
	KafkaTopic            string
	MqttBroker            string
	MqttTopicHealth       string
	LogLevel              string
}

func LoadConfig() Config {
	cfg := Config{
		UDPPort:               getEnv("UDP_PORT", "10110"),
		KafkaBootstrapServers: os.Getenv("KAFKA_BOOTSTRAP_SERVERS"),
		KafkaTopic:            getEnv("KAFKA_TOPIC", "ais.nmea"),
		MqttBroker:            getEnv("MQTT_BROKER", "tcp://localhost:1883"),
		MqttTopicHealth:       getEnv("MQTT_TOPIC_HEALTH", "field/troodos/health"),
		LogLevel:              getEnv("LOG_LEVEL", "info"),
	}

	if cfg.KafkaBootstrapServers == "" {
		fmt.Fprintln(os.Stderr, "FATAL: KAFKA_BOOTSTRAP_SERVERS environment variable is required")
		os.Exit(1)
	}

	return cfg
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
