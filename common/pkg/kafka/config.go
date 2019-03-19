package kafka

import (
	"fmt"
	flag "github.com/spf13/pflag"
	"math/rand"
	"os"
)

type SASL struct {
	Enabled  bool
	Username string
	Password string
}

type Config struct {
	Brokers      []string
	RequestTopic string
	StatusTopic  string
	ClientID     string
	GroupID      string
	Verbosity    string
	SASL         SASL
}

func DefaultGroupName() string {
	if hostname, err := os.Hostname(); err == nil {
		return fmt.Sprintf("deployd-%s", hostname)
	}
	return fmt.Sprintf("deployd-%d", rand.Int())
}

func DefaultConfig() Config {
	defaultGroup := DefaultGroupName()
	return Config{
		Verbosity:    "trace",
		Brokers:      []string{"localhost:9092"},
		RequestTopic: "deploymentRequest",
		StatusTopic:  "deploymentStatus",
		ClientID:     defaultGroup,
		GroupID:      defaultGroup,
		SASL: SASL{
			Enabled:  false,
			Username: os.Getenv("KAFKA_SASL_USERNAME"),
			Password: os.Getenv("KAFKA_SASL_PASSWORD"),
		},
	}
}

func SetupFlags(cfg *Config) {
	flag.StringSliceVar(&cfg.Brokers, "kafka-brokers", cfg.Brokers, "Comma-separated list of Kafka brokers, HOST:PORT.")
	flag.StringVar(&cfg.RequestTopic, "kafka-topic-request", cfg.RequestTopic, "Kafka topic for deployment requests.")
	flag.StringVar(&cfg.StatusTopic, "kafka-topic-status", cfg.StatusTopic, "Kafka topic for deployment statuses.")
	flag.StringVar(&cfg.ClientID, "kafka-client-id", cfg.ClientID, "Kafka client ID.")
	flag.StringVar(&cfg.GroupID, "kafka-group-id", cfg.GroupID, "Kafka consumer group ID.")
	flag.StringVar(&cfg.Verbosity, "kafka-log-verbosity", cfg.Verbosity, "Log verbosity for Kafka client.")
	flag.BoolVar(&cfg.SASL.Enabled, "kafka-sasl-enabled", cfg.SASL.Enabled, "Enable SASL authentication.")
	flag.StringVar(&cfg.SASL.Username, "kafka-sasl-username", cfg.SASL.Username, "Username for Kafka authentication.")
	flag.StringVar(&cfg.SASL.Password, "kafka-sasl-password", cfg.SASL.Password, "Password for Kafka authentication.")
}
