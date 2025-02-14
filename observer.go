package sse_server

type observer struct {
	filters      []Filter
	eventCh      chan Event
	closeOnFirst bool
	limit        int
	// emittedCount is used for tracking the number of emitted events when used with limit field
	emittedCount int
}

func (o *observer) hasPassedAllFilters(e Event) bool {
	for _, filter := range o.filters {
		if !filter(e) {
			return false
		}
	}

	return true
}

type observerBuilder struct {
	filters      []Filter
	closeOnFirst bool
	limit        int
	buffer       int
}

func NewObserverBuilder() *observerBuilder {
	return &observerBuilder{}
}

func (o *observerBuilder) NoHeartbeat() *observerBuilder {
	o.Filter(FilterNoHeartbeat)
	return o
}

func (o *observerBuilder) On(event string) *observerBuilder {
	o.Filter(func(e Event) bool {
		return e.Event != nil && *e.Event == event
	})

	return o
}

func (o *observerBuilder) Filter(filter Filter) *observerBuilder {
	if o.filters == nil {
		o.filters = make([]Filter, 0)
	}
	o.filters = append(o.filters, filter)

	return o
}

func (o *observerBuilder) First() *observerBuilder {
	o.closeOnFirst = true
	return o
}

func (o *observerBuilder) Limit(limit int) *observerBuilder {
	if limit < 1 {
		panic("limit should never be bellow 1")
	}
	o.limit = limit
	return o
}

// Buffer allows the observer to not risk and lose messages if he's slow to consume them or if you want during
// tests to consume as many messages as possible and later go through them in the same thread/process.
//
// Default buffer is 1
func (o *observerBuilder) Buffer(count int) *observerBuilder {
	if count < 0 {
		panic("buffer should never be bellow 0")
	}
	o.buffer = count
	return o
}

func (o *observerBuilder) Build() *observer {
	buffer := o.buffer
	if o.buffer < 1 {
		buffer = 1
	}
	return &observer{
		filters:      o.filters,
		limit:        o.limit,
		closeOnFirst: o.closeOnFirst,
		eventCh:      make(chan Event, buffer),
	}
}
