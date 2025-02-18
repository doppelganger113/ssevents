package sse_server

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"strings"
)

// ReadEvents - reads, typically, from an HTTP response body, constructs the event and sends it out
// to the out channel.
func ReadEvents(ctx context.Context, reader io.Reader, out chan<- Event) error {
	scanner := bufio.NewScanner(reader)
	var event Event

	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return nil
		default:
			line := scanner.Text()
			if line == "" {
				if event.Data != "" {
					select {
					case out <- event:
					case <-ctx.Done():
						return nil
					}
				}
				event = Event{} // Reset for next event
				continue
			}

			if strings.HasPrefix(line, "id: ") {
				id := strings.TrimPrefix(line, "id: ")
				event.Id = &id
			} else if strings.HasPrefix(line, "event: ") {
				evt := strings.TrimPrefix(line, "event: ")
				event.Event = &evt
			} else if strings.HasPrefix(line, "data: ") {
				event.Data += strings.TrimPrefix(line, "data: ")
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("error reading SSE stream: %w", err)
	}

	return nil
}
