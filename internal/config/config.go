package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	DatabaseURL             string
	JWTSecret               string
	HTTPAddr                string
	TelegramToken           string
	TelegramChatID          string
	KafkaBrokers            []string
	ChecksTopic             string
	ResultsTopic            string
	IncidentsTopic          string
	ClickHouseAddr          string
	ClickHouseBatchSize     int
	ClickHouseFlushInterval time.Duration
	SchedulerWorkers        int
	DetectorThreshold       int
	SchedulerTick           time.Duration
}

func Load() (*Config, error) {
	_ = godotenv.Load()

	cfg := &Config{
		DatabaseURL:    os.Getenv("DATABASE_URL"),
		JWTSecret:      os.Getenv("JWT_SECRET"),
		HTTPAddr:       getEnv("HTTP_ADDR", ":8080"),
		ClickHouseAddr: os.Getenv("CLICKHOUSE_ADDR"),
	}

	if cfg.DatabaseURL == "" {
		return nil, fmt.Errorf("DATABASE_URL is required")
	}
	if cfg.JWTSecret == "" {
		return nil, fmt.Errorf("JWT_SECRET is required")
	}

	cfg.SchedulerWorkers = getEnvInt("SCHEDULER_WORKERS", 20)
	cfg.DetectorThreshold = getEnvInt("DETECTOR_THRESHOLD", 3)
	cfg.SchedulerTick = time.Duration(getEnvInt("SCHEDULER_TICK_SECONDS", 15)) * time.Second
	brokers := getEnv("KAFKA_BROKERS", "localhost:9092")
	cfg.KafkaBrokers = strings.Split(brokers, ",")
	cfg.ClickHouseBatchSize = getEnvInt("CLICKHOUSE_BATCH_SIZE", 100)
	cfg.ClickHouseFlushInterval = time.Duration(getEnvInt("CLICKHOUSE_FLUSH_SECONDS", 5)) * time.Second
	cfg.ChecksTopic = getEnv("CHECKS_TOPIC", "checks")
	cfg.IncidentsTopic = getEnv("INCIDENTS_TOPIC", "incidents")
	cfg.ResultsTopic = getEnv("RESULTS_TOPIC", "check-results")
	cfg.TelegramToken = os.Getenv("TELEGRAM_TOKEN")
	cfg.TelegramChatID = os.Getenv("TELEGRAM_CHAT_ID")

	return cfg, nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return fallback
}
