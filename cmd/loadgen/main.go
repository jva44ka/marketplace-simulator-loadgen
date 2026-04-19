package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/jva44ka/marketplace-simulator-loadgen/internal/client"
	"github.com/jva44ka/marketplace-simulator-loadgen/internal/config"
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

	productClient := client.NewProductClient(cfg.ProductClient)
	cartClient := client.NewCartClient(cfg.CartClient)

	var wg sync.WaitGroup

	if cfg.Workers.Replenisher.Enabled {
		w := workers.NewReplenisher(cfg.Workers.Replenisher, cfg.Kafka, productClient)
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := w.Run(ctx); err != nil {
				slog.Error("replenisher exited with error", "err", err)
			}
		}()
	}

	if cfg.Workers.OrderFlow.Enabled {
		w := workers.NewOrderFlow(cfg.Workers.OrderFlow, cfg.Skus, cartClient)
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := w.Run(ctx); err != nil {
				slog.Error("order_flow exited with error", "err", err)
			}
		}()
	}

	if cfg.Workers.CartViewer.Enabled {
		w := workers.NewCartViewer(cfg.Workers.CartViewer, cartClient)
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := w.Run(ctx); err != nil {
				slog.Error("cart_viewer exited with error", "err", err)
			}
		}()
	}

	<-ctx.Done()
	slog.Info("shutting down, waiting for workers to finish...")
	wg.Wait()
	slog.Info("shutdown complete")
}
