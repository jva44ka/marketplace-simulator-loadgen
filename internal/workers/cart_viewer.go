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
	cfg        config.CartViewerConfig
	cartClient *client.CartClient
}

func NewCartViewer(
	cfg config.CartViewerConfig,
	cartClient *client.CartClient,
) *CartViewer {
	return &CartViewer{
		cfg:        cfg,
		cartClient: cartClient,
	}
}

// Run launches cfg.Parallelism goroutines that share a single rate limiter.
// Blocks until ctx is cancelled.
func (cv *CartViewer) Run(ctx context.Context) error {
	limiter := rate.NewLimiter(rate.Limit(cv.cfg.RPS), cv.cfg.RPS)

	slog.Info("cart_viewer: started", "parallelism", cv.cfg.Parallelism, "rps", cv.cfg.RPS)

	var wg sync.WaitGroup
	for i := 0; i < cv.cfg.Parallelism; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			cv.runWorker(ctx, limiter)
		}()
	}

	wg.Wait()
	return nil
}

func (cv *CartViewer) runWorker(ctx context.Context, limiter *rate.Limiter) {
	for {
		if err := limiter.Wait(ctx); err != nil {
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
