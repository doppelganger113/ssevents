package sse_server

type ObserverBuilder struct {
	filters          []Filter
	closeOnFirst     bool
	limit            int
	buffer           int
	includeHeartbeat bool
}

func NewObserverBuilder() *ObserverBuilder {
	return &ObserverBuilder{}
}

func (o *ObserverBuilder) IncludeHeartbeat() *ObserverBuilder {
	o.includeHeartbeat = true
	return o
}

func (o *ObserverBuilder) On(event string) *ObserverBuilder {
	o.Filter(func(e Event) bool {
		return e.Event != nil && *e.Event == event
	})

	return o
}

func (o *ObserverBuilder) Filter(filter Filter) *ObserverBuilder {
	if o.filters == nil {
		o.filters = make([]Filter, 0)
	}
	o.filters = append(o.filters, filter)

	return o
}

func (o *ObserverBuilder) First() *ObserverBuilder {
	o.closeOnFirst = true
	return o
}

func (o *ObserverBuilder) Limit(limit int) *ObserverBuilder {
	if limit < 1 {
		panic("limit should never be bellow 1")
	}
	o.limit = limit
	return o
}

// Buffer allows the Observer to not risk and lose messages if he's slow to consume them or if you want during
// tests to consume as many messages as possible and later go through them in the same thread/process.
//
// Default buffer is 1
func (o *ObserverBuilder) Buffer(count int) *ObserverBuilder {
	if count < 0 {
		panic("buffer should never be bellow 0")
	}
	o.buffer = count
	return o
}

func (o *ObserverBuilder) Build() *Observer {
	if !o.includeHeartbeat {
		o.Filter(FilterNoHeartbeat)
	}
	return &Observer{
		filters:      o.filters,
		limit:        o.limit,
		closeOnFirst: o.closeOnFirst,
		EventCh:      make(chan Event, o.buffer),
	}
}
