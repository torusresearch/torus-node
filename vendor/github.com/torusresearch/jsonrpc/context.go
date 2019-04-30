package jsonrpc

import (
	"context"

	"github.com/torusresearch/bijson"
)

type requestIDKey struct{}

// RequestID takes request id from context.
func RequestID(c context.Context) *bijson.RawMessage {
	return c.Value(requestIDKey{}).(*bijson.RawMessage)
}

// WithRequestID adds request id to context.
func WithRequestID(c context.Context, id *bijson.RawMessage) context.Context {
	return context.WithValue(c, requestIDKey{}, id)
}
