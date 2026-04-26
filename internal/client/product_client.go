package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/jva44ka/marketplace-simulator-loadgen/internal/config"
)

type ProductClient struct {
	baseURL   string
	authToken string
	timeout   time.Duration
	http      *http.Client
}

func NewProductClient(cfg config.ProductClientConfig) *ProductClient {
	return &ProductClient{
		baseURL:   fmt.Sprintf("http://%s:%d", cfg.Host, cfg.Port),
		authToken: cfg.AuthToken,
		timeout:   cfg.Timeout,
		http:      &http.Client{},
	}
}

type increaseCountRequest struct {
	Products []productCountItem `json:"products"`
}

type productCountItem struct {
	SKU   uint64 `json:"sku"`
	Count uint32 `json:"count"`
}

// IncreaseCount calls POST /v1/products/increase-count with X-Auth header.
func (c *ProductClient) IncreaseCount(ctx context.Context, sku uint64, count uint32) error {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	body := increaseCountRequest{
		Products: []productCountItem{{SKU: sku, Count: count}},
	}

	data, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		c.baseURL+"/v1/products/increase-count", bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Auth", c.authToken)

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("unexpected status %d. message: %s", resp.StatusCode, resp.Status)
	}

	return nil
}
