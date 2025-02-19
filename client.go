package sse_server

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"slices"
	"sync"
	"time"
)

var (
	ErrToManyFailedReconnects = errors.New("closing client due to too many reconnection attempts")
)

// Filter is a predicate like function for filtering out events consumed from the client if they should be sent
// to the observer or not.
type Filter func(e Event) bool

var FilterNoHeartbeat = func(e Event) bool {
	return e.Event != "heartbeat"
}

type ClientOptions struct {
	DropSlowConsumerMsgs bool
	Logger               *slog.Logger
}

type Client struct {
	sync.Mutex
	logger               *slog.Logger
	dropSlowConsumerMsgs bool
	client               *http.Client
	url                  string
	closed               bool
	firstConnEstablished bool
	firstConnCh          chan struct{}
	observers            []*Observer
	shutdownCtx          context.Context
	shutdownFn           context.CancelFunc
	eventCh              chan Event
	errorCh              chan error
}

// NewSSEClient connects to an SSE server and sends events to a channel
func NewSSEClient(url string, options *ClientOptions) (*Client, error) {
	var client = &http.Client{
		Timeout: 0, // No timeout
	}

	shutdownCtx, shutdownFn := context.WithCancel(context.Background())

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	var dropSlowConsumerMsgs bool

	if options != nil {
		if options.Logger != nil {
			logger = options.Logger
		}
		if options.DropSlowConsumerMsgs {
			dropSlowConsumerMsgs = true
		}
	}

	return &Client{
		dropSlowConsumerMsgs: dropSlowConsumerMsgs,
		logger:               logger,
		client:               client,
		url:                  url,
		shutdownCtx:          shutdownCtx,
		shutdownFn:           shutdownFn,
		firstConnCh:          make(chan struct{}, 1),
		eventCh:              make(chan Event),
		errorCh:              make(chan error),
	}, nil
}

// Events provides raw access to the event received from the server, though for more control you should check out
// usage of Observer
func (c *Client) Events() <-chan Event {
	return c.eventCh
}

// Errors provides access to the errors channel of the client.
func (c *Client) Errors() <-chan error {
	return c.errorCh
}

// OnError is a convenience function that allows you to react async on an error, like for example logging it.
// Note that this reads on the error channel of the client, thus reading somewhere on the Errors channel might steal
// from this consumer as well, so ensure only 1 is used.
func (c *Client) OnError(handler func(err error)) {
	go func() {
		for {
			err, ok := <-c.errorCh
			if !ok {
				return
			}
			handler(err)
		}
	}()
}

func (c *Client) isObserverDone(obs *Observer) bool {
	// First
	if obs.closeOnFirst {
		return true
	}
	// Limit
	if obs.limit > 0 {
		obs.emittedCount++
		if obs.emittedCount >= obs.limit {
			return true
		}
	}
	return false
}

func (c *Client) emitEventsWait(obs *Observer, evt Event) (isObserverDone, stop bool, err error) {
	observerTimeoutCtx := context.TODO()
	var cancel context.CancelFunc
	if obs.timeout > 0 {
		observerTimeoutCtx, cancel = context.WithTimeout(context.Background(), obs.timeout)
		defer cancel()
	}

	select {
	case obs.EventCh <- evt:
		isObserverDone = c.isObserverDone(obs)
	case <-c.shutdownCtx.Done():
		stop = true
		return
	case <-observerTimeoutCtx.Done():
		stop = true
		return
	}

	return
}

func (c *Client) emitEventsOrDrop(obs *Observer, evt Event) (isObserverDone, stop bool, err error) {
	select {
	case obs.EventCh <- evt:
		isObserverDone = c.isObserverDone(obs)
		return
	case <-c.shutdownCtx.Done():
		stop = true
		return
	default:
		c.logger.Info("Dropping event due to slow Observer", "evt", evt)
	}

	return
}

func (c *Client) fanout() {
	if len(c.observers) == 0 {
		return
	}
	for {
		evt, ok := <-c.eventCh
		if !ok {
			return
		}

		// Not going to work fully
		var obsForRemoval []*Observer

		for i := 0; i < len(c.observers); i++ {
			if c.observers[i] == nil {
				continue
			}
			if c.observers[i].hasSatisfiedFilters(evt) {
				c.logger.Debug("Consumed", "evt", evt)
				var stop bool
				var err error
				var isObserverDone bool

				if c.dropSlowConsumerMsgs {
					isObserverDone, stop, err = c.emitEventsOrDrop(c.observers[i], evt)
				} else {
					isObserverDone, stop, err = c.emitEventsWait(c.observers[i], evt)
				}
				if err != nil {
					return
				}
				if stop {
					return
				}
				if isObserverDone {
					c.logger.Debug("removing completed observer", "obs", c.observers[i])
					obsForRemoval = append(obsForRemoval, c.observers[i])
				}
			}
		}

		if obsForRemoval != nil {
			c.observers = slices.DeleteFunc(c.observers, func(o *Observer) bool {
				if slices.Contains(obsForRemoval, o) {
					close(o.EventCh)
					return true
				}
				return false
			})
			obsForRemoval = nil
		}
	}
}

// Start - event subscriber is started and blocks until it gets its first message signaling the connection started
func (c *Client) Start() {
	// run observers if any for fanout
	go c.fanout()

	go c.runReconnectionLoop(c.shutdownCtx)
	// wait for first connection
	<-c.firstConnCh
}

// Shutdown stops the client and closes all the subscribers
func (c *Client) Shutdown() {
	c.logger.Info("client shutting down")
	c.Lock()
	defer c.Unlock()
	if !c.closed {
		c.closed = true
		c.shutdownFn()
		close(c.eventCh)
		close(c.errorCh)
		for i := 0; i < len(c.observers); i++ {
			if c.observers[i] != nil {
				close(c.observers[i].EventCh)
			}
		}
	}
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

	// Notify on first connection
	if !c.firstConnEstablished {
		c.firstConnCh <- struct{}{}
	}

	return ReadEvents(ctx, resp.Body, c.eventCh)
}

func (c *Client) runReconnectionLoop(ctx context.Context) {
	defer c.Shutdown()
	var retryCounter int
	var lastTimeConnected time.Time

	for {
		// If we haven't retried recently
		if time.Since(lastTimeConnected) > 60*time.Second {
			retryCounter = 0
		}
		lastTimeConnected = time.Now()

		if err := c.connectAndListen(ctx); err != nil {
			if !c.closed {
				select {
				case c.errorCh <- err:
				default:
					c.logger.Error("dropping error, channel full", "err", err)
				}
			}
		}
		if ctx.Err() != nil {
			return
		}

		if retryCounter > 3 {
			select {
			case c.errorCh <- ErrToManyFailedReconnects:
			default:
				c.logger.Error("dropping error, channel full", "err", ErrToManyFailedReconnects)
			}
			c.Shutdown()
			return
		}

		c.logger.Info("reconnecting...")
		time.Sleep(2 * time.Second)
		retryCounter++
	}
}

// Subscribe adds the observer which will then receive the copy of the event in a fanout manner
func (c *Client) Subscribe(o *Observer) *Observer {
	if o == nil {
		panic("unable to add nil Observer")
	}
	if c.observers == nil {
		c.observers = make([]*Observer, 0)
	}
	c.observers = append(c.observers, o)

	return o
}
