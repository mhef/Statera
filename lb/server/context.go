package server

import (
	"net/http"
)

// ctxListenerKey is the type used to define the Listener key.
type ctxListenerKey struct{}

// listenerKey is the key that holds the listener through which the request arrived.
var listenerKey ctxListenerKey

// ListenerFromRequest returns the Listener indication present in the request
// context, if one. It indicate the listener through which the request arrived.
//
// Returns a bool indicating if the listener was found or not.
//
// The ok bool must be checked before using the listener.
func ListenerFromRequest(r *http.Request) (listener string, ok bool) {
	listener, ok = r.Context().Value(listenerKey).(string)
	return
}
