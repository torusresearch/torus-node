package dkgnode

import (
	"context"

	"github.com/torusresearch/bijson"
	"github.com/torusresearch/jsonrpc"
	"github.com/torusresearch/torus-node/eventbus"
)

type (
	ShareCountHandler struct {
		eventBus eventbus.Bus
	}
	ShareCountParams struct {
	}
	ShareCountResult struct {
		Count int `json:"count"`
	}
)

// For testing purposes
func (h ShareCountHandler) ServeJSONRPC(c context.Context, params *bijson.RawMessage) (interface{}, *jsonrpc.Error) {
	shareCount := NewServiceLibrary(h.eventBus, "share_count_handler").DatabaseMethods().GetShareCount()
	var res = ShareCountResult{
		Count: shareCount,
	}
	return res, nil
}
