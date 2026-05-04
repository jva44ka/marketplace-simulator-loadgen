package etcd

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/jva44ka/marketplace-simulator-loadgen/internal/config"
	clientv3 "go.etcd.io/etcd/client/v3"
	"gopkg.in/yaml.v3"
)

// NewClient creates an etcd client from EtcdConfig.
// Returns an error if the connection cannot be established within DialTimeout.
func NewClient(cfg *config.EtcdConfig) (*clientv3.Client, error) {
	dialTimeout, err := time.ParseDuration(cfg.DialTimeout)
	if err != nil {
		dialTimeout = 5 * time.Second
	}

	client, err := clientv3.New(clientv3.Config{
		Endpoints:   cfg.Endpoints,
		DialTimeout: dialTimeout,
	})
	if err != nil {
		return nil, fmt.Errorf("etcd dial: %w", err)
	}
	return client, nil
}

// ReadFromEtcd reads the config key from etcd and unmarshals it as YAML.
// Returns (cfg, true, nil) if found, (nil, false, nil) if key absent.
func ReadFromEtcd(ctx context.Context, client *clientv3.Client, key string) (*config.Config, bool, error) {
	resp, err := client.Get(ctx, key)
	if err != nil {
		return nil, false, fmt.Errorf("etcd get %q: %w", key, err)
	}
	if len(resp.Kvs) == 0 {
		return nil, false, nil
	}

	var cfg config.Config
	if err := yaml.Unmarshal(resp.Kvs[0].Value, &cfg); err != nil {
		return nil, false, fmt.Errorf("etcd unmarshal %q: %w", key, err)
	}
	return &cfg, true, nil
}

// SeedIfAbsent writes cfg (as YAML) to etcd only if the key does not exist yet.
// Uses a conditional transaction so concurrent instances won't overwrite each other.
func SeedIfAbsent(ctx context.Context, client *clientv3.Client, key string, cfg *config.Config) error {
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("yaml marshal: %w", err)
	}

	txn := client.Txn(ctx).
		If(clientv3.Compare(clientv3.Version(key), "=", 0)).
		Then(clientv3.OpPut(key, string(data)))

	resp, err := txn.Commit()
	if err != nil {
		return fmt.Errorf("etcd txn: %w", err)
	}
	if resp.Succeeded {
		slog.Info("etcd: seeded config", "key", key)
	} else {
		slog.Info("etcd: config already present, skipping seed", "key", key)
	}
	return nil
}
