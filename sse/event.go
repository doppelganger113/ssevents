package sse

import (
	"fmt"
	"strings"
)

type Event struct {
	// Id - the event ID to set the EventSource object's last event ID value.
	Id *string `json:"id,omitempty"`
	// Event - A string identifying the type of event described. If this is specified, an event will be dispatched on
	// the browser to the listener for the specified event name; the website source code should use addEventListener()
	//to listen for named events. The onmessage handler is called if no event name is specified for a message.
	Event *string `json:"event,omitempty"`
	Data  string  `json:"data"`
	// Retry, in milliseconds, specifies to the browser when it should retry the connection
	Retry *int `json:"retry,omitempty"`
}

func NewSSE(event string, data string) Event {
	return Event{Event: &event, Data: data}
}

// ToString - converts the SSEEvent into a string that will get sent as a response in the data section
func (e Event) ToString() (string, error) {
	builder := strings.Builder{}
	if e.Event != nil {
		if _, err := fmt.Fprintf(&builder, "event: %s\n", *e.Event); err != nil {
			return "", err
		}
	}

	if _, err := fmt.Fprintf(&builder, "data: %s\n", e.Data); err != nil {
		return "", err
	}

	if e.Id != nil {
		if _, err := fmt.Fprintf(&builder, "id: %s\n", *e.Id); err != nil {
			return "", err
		}
	}
	if e.Retry != nil {
		if _, err := fmt.Fprintf(&builder, "retry: %d\n", *e.Retry); err != nil {
			return "", err
		}
	}
	if _, err := builder.WriteString("\n\n"); err != nil {
		return "", err
	}

	return builder.String(), nil
}
