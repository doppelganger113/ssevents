package sse

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"
)

type Client struct {
	client *http.Client
	url    string
	closed bool
	Event  chan Event
	Error  chan error
}

func (c *Client) connectAndListen(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.url, nil)
	if err != nil {
		return fmt.Errorf("failed creating request: %w", err)
	}
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Cache-Control", "no-cache")
	req.Header.Set("Connection", "keep-alive")

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to connect: %v", err)
	}
	defer func() {
		err = errors.Join(err, resp.Body.Close())
	}()

	// Ensure the server response is SSE
	if resp.StatusCode != http.StatusOK || resp.Header.Get("Content-Type") != "text/event-stream" {
		return fmt.Errorf(
			"invalid SSE response: status %d, content-type %s",
			resp.StatusCode,
			resp.Header.Get("Content-Type"),
		)
	}

	slog.Info("Connected to SSE server...")

	return ReadEvents(ctx, resp.Body, c.Event)
}

// Start - event subscriber is start and blocks until it is done or errors
func (c *Client) Start(ctx context.Context) {
	go func() {
		defer c.close()
		var retryCounter int
		var lastTimeConnected time.Time

		for {
			// If we haven't retried recently
			if lastTimeConnected.Sub(time.Now()) > 60*time.Second {
				retryCounter = 0
			}
			lastTimeConnected = time.Now()

			if err := c.connectAndListen(ctx); err != nil {
				select {
				case c.Error <- err:
				default:
					slog.Error("dropping error, channel full", "err", err)
				}
			}
			if ctx.Err() != nil {
				return
			}
			if retryCounter > 3 {
				slog.Error("closing client due to too many reconnection attempts")
				c.close()
				return
			}

			time.Sleep(2 * time.Second)
			retryCounter++
			slog.Info("reconnecting")
		}
	}()
}

func (c *Client) close() {
	if !c.closed {
		c.closed = true
		close(c.Event)
		close(c.Error)
	}
}

func (c *Client) Shutdown() {
	c.close()
}

// NewSSEClient connects to an SSE server and sends events to a channel
func NewSSEClient(url string) (*Client, error) {
	var client = &http.Client{
		Timeout: 0, // No timeout
	}

	return &Client{
		client: client,
		url:    url,
		Event:  make(chan Event),
		Error:  make(chan error),
	}, nil
}
