package dkgnode

import (
	"context"
	"errors"
	"time"

	"github.com/tendermint/tendermint/node"
	"github.com/tendermint/tendermint/p2p"
	"github.com/tendermint/tendermint/rpc/client"
	rpcclient "github.com/tendermint/tendermint/rpc/lib/client"
	"github.com/torusresearch/torus-public/logging"
)

type BftSuite struct {
	BftRPC               *BftRPC
	BftRPCWS             *rpcclient.WSClient
	BftNode              *node.Node
	BftRPCWSQueryHandler *BftRPCWSQueryHandler
	BftRPCWSStatus       string
	TMNodeKey            *p2p.NodeKey
}

type BftRPCWS struct {
	*rpcclient.WSClient
}

func SetupBft(suite *Suite, abciServerMonitorTicker <-chan time.Time, bftWorkerMsgs chan<- string) {
	// we generate nodekey because we need it for persistent peers later. TODO: find a better way
	tmNodeKey, err := p2p.LoadOrGenNodeKey(suite.Config.BasePath + "/config/node_key.json")
	if err != nil {
		logging.Errorf("Node Key generation issue: %s", err)
	}
	bftClient := client.NewHTTP(suite.Config.BftURI, "/websocket")
	// for subscribe and unsubscribe method calls, use this
	bftClientWS := rpcclient.NewWSClient(suite.Config.BftURI, "/websocket")
	suite.BftSuite = &BftSuite{
		BftRPC:               &BftRPC{bftClient},
		BftRPCWS:             bftClientWS,
		BftRPCWSStatus:       "down",
		BftRPCWSQueryHandler: &BftRPCWSQueryHandler{make(map[string]chan []byte), make(map[string]int)},
		TMNodeKey:            tmNodeKey,
	}
	// TODO: waiting for bft to accept websocket connection
	for range abciServerMonitorTicker {
		err := bftClientWS.Start()
		if err != nil {
			logging.Errorf("COULDNOT START THE BFTWS %s", err)
		} else {
			suite.BftSuite.BftRPCWSStatus = "up"
			bftWorkerMsgs <- "bft_up"
			break
		}
	}
}

type BftRPCWSQueryHandler struct {
	QueryMap   map[string]chan []byte
	QueryCount map[string]int
}

func (bftSuite *BftSuite) RegisterQuery(query string, count int) (chan []byte, error) {
	logging.Debugf("BFTWS: registering query %s", query)
	if bftSuite.BftRPCWSQueryHandler.QueryMap[query] != nil {
		return nil, errors.New("BFTWS: query has already been registered for query: " + query)
	}
	ctx := context.Background()
	err := bftSuite.BftRPCWS.Subscribe(ctx, query)
	if err != nil {
		return nil, err
	}
	responseCh := make(chan []byte, 1)
	bftSuite.BftRPCWSQueryHandler.QueryMap[query] = responseCh
	bftSuite.BftRPCWSQueryHandler.QueryCount[query] = count
	return responseCh, nil
}

func (bftSuite *BftSuite) DeregisterQuery(query string) error {
	logging.Debugf("BFTWS: deregistering query %s", query)
	ctx := context.Background()
	err := bftSuite.BftRPCWS.Unsubscribe(ctx, query)
	if err != nil {
		logging.Debugf("BFTWS: websocket could not unsubscribe, queryString: %s", query)
		return err
	}
	if responseCh, found := bftSuite.BftRPCWSQueryHandler.QueryMap[query]; found {
		delete(bftSuite.BftRPCWSQueryHandler.QueryMap, query)
		close(responseCh)
	}
	if _, found := bftSuite.BftRPCWSQueryHandler.QueryCount[query]; found {
		delete(bftSuite.BftRPCWSQueryHandler.QueryCount, query)
	}
	return nil
}
