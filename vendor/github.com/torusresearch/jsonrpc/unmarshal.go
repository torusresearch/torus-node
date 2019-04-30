package jsonrpc

import (
	"github.com/torusresearch/bijson"
)

// Unmarshal decodes JSON-RPC params.
func Unmarshal(params *bijson.RawMessage, dst interface{}) *Error {
	if params == nil {
		return ErrInvalidParams()
	}
	if err := bijson.Unmarshal(*params, dst); err != nil {
		return ErrInvalidParams()
	}
	return nil
}
