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

type CartClient struct {
	baseURL string
	timeout time.Duration
	http    *http.Client
}

func NewCartClient(cfg config.CartClientConfig) *CartClient {
	return &CartClient{
		baseURL: fmt.Sprintf("http://%s:%d", cfg.Host, cfg.Port),
		timeout: cfg.Timeout,
		http:    &http.Client{},
	}
}

type addToCartRequest struct {
	Count uint32 `json:"count"`
}

// AddToCart calls POST /user/{userId}/cart/{sku}.
func (c *CartClient) AddToCart(ctx context.Context, userId string, sku uint64, count uint32) error {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	body := addToCartRequest{Count: count}

	data, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/user/%s/cart/%d", c.baseURL, userId, sku)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("unexpected status %d", resp.StatusCode)
	}

	return nil
}

// Checkout calls POST /user/{userId}/cart/checkout.
func (c *CartClient) Checkout(ctx context.Context, userId string) error {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	url := fmt.Sprintf("%s/user/%s/cart/checkout", c.baseURL, userId)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, http.NoBody)
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("unexpected status %d", resp.StatusCode)
	}

	return nil
}

// CleanCart calls DELETE /user/{userId}/cart.
func (c *CartClient) CleanCart(ctx context.Context, userId string) error {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	url := fmt.Sprintf("%s/user/%s/cart", c.baseURL, userId)
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, url, http.NoBody)
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("unexpected status %d", resp.StatusCode)
	}

	return nil
}

// GetCart calls GET /user/{userId}/cart.
func (c *CartClient) GetCart(ctx context.Context, userId string) error {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	url := fmt.Sprintf("%s/user/%s/cart", c.baseURL, userId)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("unexpected status %d", resp.StatusCode)
	}

	return nil
}
