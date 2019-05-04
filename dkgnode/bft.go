package dkgnode

import (
	"context"
	"errors"
	"time"

	"github.com/tendermint/tendermint/node"
	"github.com/tendermint/tendermint/rpc/client"
	rpcclient "github.com/tendermint/tendermint/rpc/lib/client"
	"github.com/tidwall/gjson"
	"github.com/torusresearch/torus-public/logging"
)

type BftSuite struct {
	BftRPC               *BftRPC
	BftRPCWS             *rpcclient.WSClient
	BftNode              *node.Node
	BftRPCWSQueryHandler *BftRPCWSQueryHandler
	BftRPCWSStatus       string
}

type BftRPCWS struct {
	*rpcclient.WSClient
}

func bftWorker(suite *Suite, bftWorkerMsgs <-chan string) {
	logging.Debug("BFTWS: listening to responsesCh")
	for bftWorkerMsg := range bftWorkerMsgs {
		if bftWorkerMsg == "bft_up" {
			break
		}
	}
	for e := range suite.BftSuite.BftRPCWS.ResponsesCh {
		queryString := gjson.GetBytes(e.Result, "query").String()
		logging.Debugf("BFTWS: query %s", queryString)
		if e.Error != nil {
			logging.Errorf("BFTWS: websocket subscription received error: %s", e.Error.Error())
		}
		if suite.BftSuite.BftRPCWSQueryHandler.QueryMap[queryString] == nil {
			logging.Debugf("BFTWS: websocket subscription received message but no listener, querystring: %s", queryString)
			continue
		}
		suite.BftSuite.BftRPCWSQueryHandler.QueryMap[queryString] <- e.Result
		if suite.BftSuite.BftRPCWSQueryHandler.QueryCount[queryString] != 0 {
			suite.BftSuite.BftRPCWSQueryHandler.QueryCount[queryString] = suite.BftSuite.BftRPCWSQueryHandler.QueryCount[queryString] - 1
			if suite.BftSuite.BftRPCWSQueryHandler.QueryCount[queryString] == 0 {
				close(suite.BftSuite.BftRPCWSQueryHandler.QueryMap[queryString])
				delete(suite.BftSuite.BftRPCWSQueryHandler.QueryMap, queryString)
				ctx := context.Background()
				err := suite.BftSuite.BftRPCWS.Unsubscribe(ctx, queryString)
				if err != nil {
					logging.Errorf("BFTWS: websocket could not unsubscribe, queryString: %s", queryString)
				}
				delete(suite.BftSuite.BftRPCWSQueryHandler.QueryCount, queryString)
			}
		}
	}

}

func SetupBft(suite *Suite, abciServerMonitorTicker <-chan time.Time, bftWorkerMsgs chan<- string) {
	// we generate nodekey because we need it for persistent peers later. TODO: find a better way
	suite.BftSuite = &BftSuite{
		BftRPC:               nil,
		BftRPCWS:             nil,
		BftRPCWSStatus:       "down",
		BftRPCWSQueryHandler: &BftRPCWSQueryHandler{make(map[string]chan []byte), make(map[string]int)},
	}
	// TODO: waiting for bft to accept websocket connection
	for range abciServerMonitorTicker {
		bftClient := client.NewHTTP(suite.Config.BftURI, "/websocket")
		// for subscribe and unsubscribe method calls, use this
		bftClientWS := rpcclient.NewWSClient(suite.Config.BftURI, "/websocket")
		err := bftClientWS.Start()
		if err != nil {
			logging.Errorf("COULDNOT START THE BFTWS %s", err)
		} else {
			suite.BftSuite.BftRPC = &BftRPC{bftClient}
			suite.BftSuite.BftRPCWS = bftClientWS
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
