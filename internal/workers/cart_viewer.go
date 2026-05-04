package workers

import (
	"context"
	"log/slog"
	"sync"

	"golang.org/x/time/rate"

	"github.com/jva44ka/marketplace-simulator-loadgen/internal/client"
	"github.com/jva44ka/marketplace-simulator-loadgen/internal/config"
)

// CartViewer simulates users fetching their cart contents.
type CartViewer struct {
	cfgStore   *config.ConfigStore
	cartClient *client.CartClient
	Limiter    *rate.Limiter // exported so main can call SetLimit/SetBurst on hot-reload
}

func NewCartViewer(
	cfgStore *config.ConfigStore,
	cartClient *client.CartClient,
) *CartViewer {
	cfg := cfgStore.Load().Workers.CartViewer
	lim := rate.NewLimiter(rate.Limit(cfg.RPS), cfg.RPS)
	return &CartViewer{
		cfgStore:   cfgStore,
		cartClient: cartClient,
		Limiter:    lim,
	}
}

// Run launches cfg.Parallelism goroutines that share a single rate limiter.
// Blocks until ctx is cancelled.
func (cv *CartViewer) Run(ctx context.Context) error {
	cfg := cv.cfgStore.Load().Workers.CartViewer
	slog.Info("cart_viewer: started", "parallelism", cfg.Parallelism, "rps", cfg.RPS)

	var wg sync.WaitGroup
	for i := 0; i < cfg.Parallelism; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			cv.runWorker(ctx)
		}()
	}

	wg.Wait()
	return nil
}

func (cv *CartViewer) runWorker(ctx context.Context) {
	for {
		if err := cv.Limiter.Wait(ctx); err != nil {
			// ctx cancelled
			return
		}

		// Use a random userId so requests are spread across users.
		userId := newUUID()

		if err := cv.cartClient.GetCart(ctx, userId); err != nil {
			slog.Error("cart_viewer: get cart failed", "user_id", userId, "err", err)
			continue
		}
		slog.Info("cart_viewer: got cart", "user_id", userId)
	}
}
