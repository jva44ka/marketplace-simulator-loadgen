package config

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	ProductClient ProductClientConfig `yaml:"product-client"`
	CartClient    CartClientConfig    `yaml:"cart-client"`
	Kafka         KafkaConfig         `yaml:"kafka"`
	SkuRange      SkuRangeConfig      `yaml:"sku-range"`
	Workers       WorkersConfig       `yaml:"workers"`
	Etcd          *EtcdConfig         `yaml:"etcd"`
}

type EtcdConfig struct {
	Endpoints   []string `yaml:"endpoints"`
	DialTimeout string   `yaml:"dial-timeout"`
	ConfigKey   string   `yaml:"config-key"`
}

type SkuRangeConfig struct {
	Min uint64 `yaml:"min"`
	Max uint64 `yaml:"max"`
}

func (s SkuRangeConfig) GetSkus() []uint64 {
	if s.Max < s.Min {
		return nil
	}
	skus := make([]uint64, 0, s.Max-s.Min+1)
	for sku := s.Min; sku <= s.Max; sku++ {
		skus = append(skus, sku)
	}
	return skus
}

type ProductClientConfig struct {
	Host      string        `yaml:"host"`
	Port      int           `yaml:"port"`
	AuthToken string        `yaml:"auth-token"`
	Timeout   time.Duration `yaml:"timeout"`
}

type CartClientConfig struct {
	Host    string        `yaml:"host"`
	Port    int           `yaml:"port"`
	Timeout time.Duration `yaml:"timeout"`
}

type KafkaConfig struct {
	Brokers            []string `yaml:"brokers"`
	ProductEventsTopic string   `yaml:"product-events-topic"`
	ConsumerGroup      string   `yaml:"consumer-group"`
}

type WorkersConfig struct {
	Replenisher ReplenisherConfig `yaml:"replenisher"`
	OrderFlow   OrderFlowConfig   `yaml:"order-flow"`
	CartViewer  CartViewerConfig  `yaml:"cart-viewer"`
}

type ReplenisherConfig struct {
	Enabled           bool   `yaml:"enabled"`
	Parallelism       int    `yaml:"parallelism"`
	LowStockThreshold int    `yaml:"low-stock-threshold"`
	ReplenishCount    uint32 `yaml:"replenish-count"`
}

type OrderFlowConfig struct {
	Enabled     bool `yaml:"enabled"`
	Parallelism int  `yaml:"parallelism"`
	RPS         int  `yaml:"rps"`
}

type CartViewerConfig struct {
	Enabled     bool `yaml:"enabled"`
	Parallelism int  `yaml:"parallelism"`
	RPS         int  `yaml:"rps"`
}

func Load() (*Config, error) {
	path := os.Getenv("CONFIG_PATH")
	if path == "" {
		path = "configs/values_local.yaml"
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config file %q: %w", path, err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config file %q: %w", path, err)
	}

	return &cfg, nil
}
