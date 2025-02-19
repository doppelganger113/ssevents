package ssevents

import (
	"context"
	"time"
)

type Observer struct {
	EventCh      chan Event
	filters      []Filter
	closeOnFirst bool
	limit        int
	// emittedCount is used for tracking the number of emitted events when used with limit field
	emittedCount int
	timeout      time.Duration
}

func (o *Observer) hasSatisfiedFilters(e Event) bool {
	for _, filter := range o.filters {
		if !filter(e) {
			return false
		}
	}

	return true
}

// WaitForAll blocks and starts reading from the observer until it has completed, returning all events as a result.
func (o *Observer) WaitForAll() []Event {
	var events []Event

	for evt := range o.EventCh {
		events = append(events, evt)
	}

	return events
}

// WaitForAllOrTimeout is identical to the WaitForAll except that it times out after a given duration.
func (o *Observer) WaitForAllOrTimeout(timeout time.Duration) ([]Event, error) {
	eventsCh := make(chan []Event)

	go func() {
		var events []Event
		for evt := range o.EventCh {
			events = append(events, evt)
		}
		eventsCh <- events
		defer close(eventsCh)
	}()

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	select {
	case events := <-eventsCh:
		return events, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}
