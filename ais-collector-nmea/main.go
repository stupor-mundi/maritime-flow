package main

import (
	"context"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"sync/atomic"
	"syscall"
	"time"
)

func main() {
	cfg := LoadConfig()
	setupLogger(cfg.LogLevel)

	slog.Info("starting ais-collector-nmea", "udpPort", cfg.UDPPort, "kafkaTopic", cfg.KafkaTopic)

	producer, err := NewKafkaProducer(cfg)
	if err != nil {
		slog.Error("failed to create kafka producer", "error", err)
		os.Exit(1)
	}

	mqttClient := NewMqttClient(cfg)

	conn, err := net.ListenPacket("udp4", ":"+cfg.UDPPort)
	if err != nil {
		slog.Error("failed to listen on UDP port", "port", cfg.UDPPort, "error", err)
		os.Exit(1)
	}

	var messageCount, errorCount int64
	var lastMessageTime atomic.Value
	lastMessageTime.Store(time.Time{})
	startTime := time.Now()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go func() {
		ticker := time.NewTicker(60 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				PublishHealth(mqttClient, cfg, HealthStats{
					MessageCount:    atomic.LoadInt64(&messageCount),
					ErrorCount:      atomic.LoadInt64(&errorCount),
					LastMessageTime: lastMessageTime.Load().(time.Time),
					UptimeSeconds:   int64(time.Since(startTime).Seconds()),
				})
			}
		}
	}()

	go func() {
		<-ctx.Done()
		slog.Info("shutdown signal received, flushing producer")
		producer.Flush(5000)
		if mqttClient != nil {
			mqttClient.Disconnect(250)
		}
		conn.Close()
		slog.Info("shutdown complete")
		os.Exit(0)
	}()

	buf := make([]byte, 512)
	for {
		n, _, err := conn.ReadFrom(buf)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			slog.Error("udp read error", "error", err)
			atomic.AddInt64(&errorCount, 1)
			continue
		}

		sentence := string(buf[:n])
		slog.Debug("received message", "sentence", sentence)

		if !ValidateNmea(sentence) {
			slog.Warn("dropping invalid NMEA sentence", "sentence", sentence)
			atomic.AddInt64(&errorCount, 1)
			continue
		}

		mmsi := ExtractMmsi(sentence)
		Produce(producer, cfg, mmsi, sentence)

		atomic.AddInt64(&messageCount, 1)
		lastMessageTime.Store(time.Now())
	}
}

func setupLogger(level string) {
	var lvl slog.Level
	switch level {
	case "debug":
		lvl = slog.LevelDebug
	case "warn":
		lvl = slog.LevelWarn
	case "error":
		lvl = slog.LevelError
	default:
		lvl = slog.LevelInfo
	}

	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: lvl})))
}
