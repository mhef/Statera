package lb

import (
	"net/http"
)

// Mux is a HTTP handler that can chain different handlers. When a request arrives,
// Mux make it pass in order through each of the chained handlers.
type Mux struct {
	handlers []func(http.Handler) http.Handler
}

// NewMux creates and return a new instance of Mux.
//
// Currently, Mux dont need initialization, however, in the future it may need.
// By that, is advised to use this func to create a new Mux and not creating it
// directly.
func NewMux() *Mux {
	return &Mux{}
}

// Chain appends h to the request handling chain.
//
// h takes a handler that when is called passes the request to the next handler
// of the chain. If h return before calling the next handler, the passage through
// the chain is aborted.
func (m *Mux) Chain(h func(http.Handler) http.Handler) {
	m.handlers = append(m.handlers, h)
}

// endpoint is the last handler of the chain.
//
// This handler is necessary because every chain handler takes a next handler.
// By that, the last chain handler needs a next handler, taking to this func.
//
// Currently, this func is empty, however, it may be implemented in the future.
func endpoint(w http.ResponseWriter, r *http.Request) {}

func (m *Mux) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if len(m.handlers) < 2 {
		panic("lb/mux: at least 2 chained handlers are necessary to use mux")
	}

	f := m.handlers[len(m.handlers)-1](http.HandlerFunc(endpoint))
	for i := len(m.handlers) - 2; i >= 0; i-- {
		f = m.handlers[i](f)
	}
	f.ServeHTTP(w, r)
}
