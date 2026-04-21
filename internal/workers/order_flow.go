package workers

import (
	"context"
	"fmt"
	"log/slog"
	"math/rand"
	"sync"

	"golang.org/x/time/rate"

	"github.com/jva44ka/marketplace-simulator-loadgen/internal/client"
	"github.com/jva44ka/marketplace-simulator-loadgen/internal/config"
)

// OrderFlow simulates users adding items to their carts and checking out.
type OrderFlow struct {
	cfg        config.OrderFlowConfig
	skus       []uint64
	cartClient *client.CartClient
}

func NewOrderFlow(
	cfg config.OrderFlowConfig,
	skus []uint64,
	cartClient *client.CartClient,
) *OrderFlow {
	return &OrderFlow{
		cfg:        cfg,
		skus:       skus,
		cartClient: cartClient,
	}
}

// Run launches cfg.Parallelism goroutines that share a single rate limiter.
// Blocks until ctx is cancelled.
func (o *OrderFlow) Run(ctx context.Context) error {
	limiter := rate.NewLimiter(rate.Limit(o.cfg.RPS), o.cfg.RPS)

	slog.Info("order_flow: started", "parallelism", o.cfg.Parallelism, "rps", o.cfg.RPS)

	var wg sync.WaitGroup
	for i := 0; i < o.cfg.Parallelism; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			o.runWorker(ctx, limiter)
		}()
	}

	wg.Wait()
	return nil
}

func (o *OrderFlow) runWorker(ctx context.Context, limiter *rate.Limiter) {
	for {
		if err := limiter.Wait(ctx); err != nil {
			// ctx cancelled
			return
		}

		userId := newUUID()
		sku := o.skus[rand.Intn(len(o.skus))]

		if err := o.cartClient.AddToCart(ctx, userId, sku, 1); err != nil {
			slog.Error("order_flow: add to cart failed",
				"user_id", userId, "sku", sku, "err", err)
			continue
		}
		slog.Info("order_flow: added to cart", "user_id", userId, "sku", sku)

		if err := o.cartClient.Checkout(ctx, userId); err != nil {
			slog.Error("order_flow: checkout failed", "user_id", userId, "err", err)
			if cleanErr := o.cartClient.CleanCart(ctx, userId); cleanErr != nil {
				slog.Error("order_flow: clean cart failed", "user_id", userId, "err", cleanErr)
			} else {
				slog.Info("order_flow: cart cleaned after failed checkout", "user_id", userId)
			}
			continue
		}
		slog.Info("order_flow: checkout completed", "user_id", userId, "sku", sku)
	}
}

// newUUID returns a random UUID v4 string without external dependencies.
func newUUID() string {
	var b [16]byte
	rand.Read(b[:]) //nolint:gosec
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}
