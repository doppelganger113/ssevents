package sse_server

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"slices"
	"time"
)

type Filter func(e Event) bool

var FilterNoHeartbeat = func(e Event) bool {
	return e.Event == nil || *e.Event != "heartbeat"
}

type Client struct {
	client               *http.Client
	url                  string
	closed               bool
	firstConnEstablished bool
	firstConnCh          chan struct{}
	observers            []*observer
	Event                chan Event
	Error                chan error
}

// NewSSEClient connects to an SSE server and sends events to a channel
func NewSSEClient(url string) (*Client, error) {
	var client = &http.Client{
		Timeout: 0, // No timeout
	}

	return &Client{
		client:      client,
		url:         url,
		firstConnCh: make(chan struct{}, 1),
		Event:       make(chan Event),
		Error:       make(chan error),
	}, nil
}

func (c *Client) removeObserver(index int) {
	close(c.observers[index].eventCh)
	c.observers = slices.Delete(c.observers, index, index+1)
}

func (c *Client) fanout() {
	if c.observers == nil || len(c.observers) == 0 {
		return
	}
	for {
		evt, ok := <-c.Event
		if !ok {
			return
		}

		// Not going to work fully
		var rememberWhichToRemove []*observer

		for i := 0; i < len(c.observers); i++ {
			if c.observers[i] == nil {
				continue
			}
			if c.observers[i].hasPassedAllFilters(evt) {

				select {
				case c.observers[i].eventCh <- evt:
					// First
					if c.observers[i].closeOnFirst {
						fmt.Println("Added for removal", c.observers[i])
						rememberWhichToRemove = append(rememberWhichToRemove, c.observers[i])
						//c.removeObserver(i)
					}
					// Limit
					if c.observers[i].limit > 0 {
						c.observers[i].emittedCount++
						if c.observers[i].emittedCount >= c.observers[i].limit {
							fmt.Println("Added for removal", c.observers[i])
							//c.removeObserver(i)
							rememberWhichToRemove = append(rememberWhichToRemove, c.observers[i])
						}
					}
				}
			}
		}

		if rememberWhichToRemove != nil {
			slices.DeleteFunc(c.observers, func(o *observer) bool {
				if slices.Contains(rememberWhichToRemove, o) {
					fmt.Printf("Removing observer %+v\n", o)
					close(o.eventCh)
					return true
				}
				return false
			})
			rememberWhichToRemove = nil
		}
	}
}

// Start - event subscriber is started and blocks until it gets its first message signaling the connection started
func (c *Client) Start(ctx context.Context) {
	// run observers if any for fanout
	go c.fanout()

	go c.runReconnectionLoop(ctx)
	// wait for first connection
	<-c.firstConnCh
}

func (c *Client) close() {
	fmt.Println("closing client...")
	if !c.closed {
		c.closed = true
		close(c.Event)
		close(c.Error)
		for i := 0; i < len(c.observers); i++ {
			if c.observers[i] != nil {
				close(c.observers[i].eventCh)
			}
		}
	}
}

func (c *Client) Shutdown() {
	c.close()
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

	return ReadEvents(ctx, resp.Body, c.Event)
}

func (c *Client) runReconnectionLoop(ctx context.Context) {
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
			if !c.closed {
				select {
				case c.Error <- err:
				default:
					slog.Error("dropping error, channel full", "err", err)
				}
			}
		}
		fmt.Println("Connection broken")
		if ctx.Err() != nil {
			return
		}
		fmt.Println("Checking for retry count")
		if retryCounter > 3 {
			slog.Error("closing client due to too many reconnection attempts")
			c.close()
			return
		}

		time.Sleep(2 * time.Second)
		retryCounter++
		slog.Info("reconnecting")
	}
}

func (c *Client) Subscribe(o *observer) <-chan Event {
	if o == nil {
		panic("unable to add nil observer")
	}
	if c.observers == nil {
		c.observers = make([]*observer, 0)
	}
	c.observers = append(c.observers, o)

	return o.eventCh
}

//func (c *Client) Filter(filter func(e Event) bool) <-chan Event {
//	filterCh := make(chan Event)
//
//	go func() {
//		for {
//			evt, ok := <-c.Event
//			if !ok {
//				close(filterCh)
//				return
//			}
//			if filter(evt) {
//				filterCh <- evt
//			}
//		}
//	}()
//
//	return filterCh
//}

//type bserver struct {
//	filters []Filter
//	eventCh chan Event
//}
//
//func (c *Client) AddObserver(filters ...Filter) <-chan Event {
//	obs := &observer{
//		eventCh: make(chan Event, 1),
//		filters: filters,
//	}
//	if c.observers == nil {
//		c.observers = make([]*observer, 1)
//	}
//	c.observers = append(c.observers, obs)
//
//	return obs.eventCh
//}
//
//func (c *Client) AddObserverOnEvent(event string, filters ...Filter) <-chan Event {
//	filterByEvent := func(e Event) bool {
//		return e.Event != nil && *e.Event == event
//	}
//	updatedFilters := append(filters, filterByEvent)
//	return c.AddObserver(updatedFilters...)
//}
//
//func (c *Client) WaitForFirstEvent(filters ...Filter) {
//
//}
