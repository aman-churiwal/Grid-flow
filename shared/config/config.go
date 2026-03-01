package config

import (
	"errors"
	"fmt"

	"github.com/spf13/viper"
)

type Config struct {
	ServiceName    string   `mapstructure:"SERVICE_NAME"`
	Port           int      `mapstructure:"PORT"`
	KafkaBrokers   []string `mapstructure:"KAFKA_BROKERS"`
	RedisAddr      string   `mapstructure:"REDIS_ADDR"`
	PostgresDSN    string   `mapstructure:"POSTGRES_DSN"`
	EtcdEndpoints  []string `mapstructure:"ETCD_ENDPOINTS"`
	JaegerEndpoint string   `mapstructure:"JAEGER_ENDPOINT"`
	AIEndpoint     string   `mapstructure:"AI_ENDPOINT"`
}

func Load() (c Config, err error) {
	viper.AddConfigPath("./")
	viper.SetConfigName(".env")
	viper.SetConfigType("env")

	viper.AutomaticEnv()

	err = viper.ReadInConfig()

	if err != nil {
		var notFoundErr viper.ConfigFileNotFoundError
		if !errors.As(err, &notFoundErr) {
			return c, err
		}
	}

	err = viper.Unmarshal(&c)
	if err != nil {
		return c, err
	}

	if c.ServiceName == "" {
		return c, fmt.Errorf("SERVICE_NAME is required")
	}
	if c.Port == 0 {
		return c, fmt.Errorf("PORT is required")
	}
	if len(c.KafkaBrokers) == 0 {
		return c, fmt.Errorf("KAFKA_BROKERS is required")
	}
	if c.RedisAddr == "" {
		return c, fmt.Errorf("REDIS_ADDR is required")
	}
	if c.PostgresDSN == "" {
		return c, fmt.Errorf("POSTGRES_DSN is required")
	}
	if len(c.EtcdEndpoints) == 0 {
		return c, fmt.Errorf("ETCD_ENDPOINTS is required")
	}
	if c.JaegerEndpoint == "" {
		return c, fmt.Errorf("JAEGER_ENDPOINT is required")
	}
	if c.AIEndpoint == "" {
		return c, fmt.Errorf("AI_ENDPOINT is required")
	}

	return c, nil
}
