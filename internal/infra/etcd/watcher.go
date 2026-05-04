package etcd

import (
	"context"
	"log/slog"
	"slices"

	"github.com/jva44ka/marketplace-simulator-loadgen/internal/config"
	clientv3 "go.etcd.io/etcd/client/v3"
	"gopkg.in/yaml.v3"
)

// Watch subscribes to changes on key and applies them to store.
// onChange is called after every successful update (may be nil).
// Blocks until ctx is cancelled or the watch channel closes.
func Watch(
	ctx context.Context,
	client *clientv3.Client,
	key string,
	store *config.ConfigStore,
	onChange func(*config.Config),
) {
	watchChan := client.Watch(ctx, key)
	for {
		select {
		case <-ctx.Done():
			return
		case resp, ok := <-watchChan:
			if !ok {
				slog.Warn("etcd: watch channel closed")
				return
			}
			for _, ev := range resp.Events {
				if ev.Type != clientv3.EventTypePut {
					continue
				}
				var newCfg config.Config
				if err := yaml.Unmarshal(ev.Kv.Value, &newCfg); err != nil {
					slog.Error("etcd: cannot unmarshal new config", "err", err)
					continue
				}
				warnIfRestartRequired(store.Load(), &newCfg)
				store.Store(&newCfg)
				slog.Info("etcd: config updated")
				if onChange != nil {
					onChange(&newCfg)
				}
			}
		}
	}
}

// warnIfRestartRequired logs a warning when fields that require a restart change.
func warnIfRestartRequired(old, new *config.Config) {
	if old.ProductClient.Host != new.ProductClient.Host ||
		old.ProductClient.Port != new.ProductClient.Port {
		slog.Warn("etcd: product-client host/port changed — restart required to reconnect")
	}
	if old.CartClient.Host != new.CartClient.Host ||
		old.CartClient.Port != new.CartClient.Port {
		slog.Warn("etcd: cart-client host/port changed — restart required to reconnect")
	}
	if !slices.Equal(old.Kafka.Brokers, new.Kafka.Brokers) ||
		old.Kafka.ProductEventsTopic != new.Kafka.ProductEventsTopic ||
		old.Kafka.ConsumerGroup != new.Kafka.ConsumerGroup {
		slog.Warn("etcd: kafka config changed — restart required to reconnect")
	}
	if old.SkuRange.Min != new.SkuRange.Min || old.SkuRange.Max != new.SkuRange.Max {
		slog.Warn("etcd: sku-range changed — restart required to take effect")
	}
}
