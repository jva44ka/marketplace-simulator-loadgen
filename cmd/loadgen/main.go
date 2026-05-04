package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"golang.org/x/time/rate"

	"github.com/jva44ka/marketplace-simulator-loadgen/internal/client"
	"github.com/jva44ka/marketplace-simulator-loadgen/internal/config"
	etcdPkg "github.com/jva44ka/marketplace-simulator-loadgen/internal/infra/etcd"
	"github.com/jva44ka/marketplace-simulator-loadgen/internal/workers"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()

	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	cfg, err := config.Load()
	if err != nil {
		slog.Error("load config", "err", err)
		os.Exit(1)
	}

	// --- ConfigStore: starts from yaml ---
	cfgStore := config.NewConfigStore(cfg)

	// applyDynamicConfig is populated after workers are created so the closure
	// can capture their limiter references.
	var applyDynamicConfig func(*config.Config)

	// --- etcd: connect, load or seed, then watch ---
	if cfg.Etcd != nil {
		etcdClient, err := etcdPkg.NewClient(cfg.Etcd)
		if err != nil {
			slog.Warn("etcd: failed to connect, using yaml defaults", "err", err)
		} else {
			initCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			if etcdCfg, found, err := etcdPkg.ReadFromEtcd(initCtx, etcdClient, cfg.Etcd.ConfigKey); err != nil {
				slog.Warn("etcd: failed to read config, using yaml defaults", "err", err)
			} else if found {
				cfgStore.Store(etcdCfg)
				slog.Info("etcd: loaded config from etcd")
			} else {
				if err := etcdPkg.SeedIfAbsent(initCtx, etcdClient, cfg.Etcd.ConfigKey, cfg); err != nil {
					slog.Warn("etcd: failed to seed config", "err", err)
				}
			}

			// Watch runs in the background; it will call applyDynamicConfig once
			// it is set (after workers are created below).
			go func() {
				etcdPkg.Watch(ctx, etcdClient, cfg.Etcd.ConfigKey, cfgStore, func(newCfg *config.Config) {
					if applyDynamicConfig != nil {
						applyDynamicConfig(newCfg)
					}
				})
				if err := etcdClient.Close(); err != nil {
					slog.Warn("etcd: close client", "err", err)
				}
			}()
		}
	}

	// --- Build clients ---
	productClient := client.NewProductClient(cfgStore.Load().ProductClient)
	cartClient := client.NewCartClient(cfgStore.Load().CartClient)

	// --- Start workers ---
	var (
		orderFlow   *workers.OrderFlow
		cartViewer  *workers.CartViewer
		replenisher *workers.Replenisher
	)

	var wg sync.WaitGroup
	workersCfg := cfgStore.Load().Workers

	if workersCfg.Replenisher.Enabled {
		replenisher = workers.NewReplenisher(cfgStore, productClient)
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := replenisher.Run(ctx); err != nil {
				slog.Error("replenisher exited with error", "err", err)
			}
		}()
	}

	if workersCfg.OrderFlow.Enabled {
		orderFlow = workers.NewOrderFlow(cfgStore, cfgStore.Load().SkuRange.GetSkus(), cartClient)
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := orderFlow.Run(ctx); err != nil {
				slog.Error("order_flow exited with error", "err", err)
			}
		}()
	}

	if workersCfg.CartViewer.Enabled {
		cartViewer = workers.NewCartViewer(cfgStore, cartClient)
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := cartViewer.Run(ctx); err != nil {
				slog.Error("cart_viewer exited with error", "err", err)
			}
		}()
	}

	// Wire up the dynamic config callback now that workers (and their limiters) exist.
	applyDynamicConfig = func(newCfg *config.Config) {
		if orderFlow != nil {
			rps := newCfg.Workers.OrderFlow.RPS
			orderFlow.Limiter.SetLimit(rate.Limit(rps))
			orderFlow.Limiter.SetBurst(rps)
			slog.Info("dynamic config: order-flow rps updated", "rps", rps)
		}
		if cartViewer != nil {
			rps := newCfg.Workers.CartViewer.RPS
			cartViewer.Limiter.SetLimit(rate.Limit(rps))
			cartViewer.Limiter.SetBurst(rps)
			slog.Info("dynamic config: cart-viewer rps updated", "rps", rps)
		}
		// replenisher reads cfgStore inline on every Kafka message — no action needed here.
		_ = replenisher
	}

	<-ctx.Done()
	slog.Info("shutting down, waiting for workers to finish...")
	wg.Wait()
	slog.Info("shutdown complete")
}
