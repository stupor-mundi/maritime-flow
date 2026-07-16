package main

import (
	"encoding/json"
	"log/slog"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

type HealthStats struct {
	MessageCount    int64     `json:"messageCount"`
	ErrorCount      int64     `json:"errorCount"`
	LastMessageTime time.Time `json:"lastMessageTime"`
	UptimeSeconds   int64     `json:"uptimeSeconds"`
}

func NewMqttClient(cfg Config) mqtt.Client {
	opts := mqtt.NewClientOptions().
		AddBroker(cfg.MqttBroker).
		SetClientID("ais-collector-nmea")

	client := mqtt.NewClient(opts)
	if token := client.Connect(); token.Wait() && token.Error() != nil {
		slog.Warn("failed to connect to MQTT broker", "broker", cfg.MqttBroker, "error", token.Error())
		return nil
	}

	return client
}

func PublishHealth(client mqtt.Client, cfg Config, stats HealthStats) {
	if client == nil {
		return
	}

	payload, err := json.Marshal(stats)
	if err != nil {
		slog.Error("failed to marshal health stats", "error", err)
		return
	}

	token := client.Publish(cfg.MqttTopicHealth, 0, false, payload)
	token.Wait()
	if token.Error() != nil {
		slog.Warn("failed to publish health stats", "error", token.Error())
	}
}
