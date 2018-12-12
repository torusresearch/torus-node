package dkgnode

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/tendermint/tendermint/node"
	"github.com/tendermint/tendermint/rpc/client"
	rpcclient "github.com/tendermint/tendermint/rpc/lib/client"
	"github.com/tidwall/gjson"
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

func SetUpBft(suite *Suite) {

	bftClient := client.NewHTTP(suite.Config.BftURI, "/websocket")

	// for subscribe and unsubscribe method calls, use this
	bftClientWS := rpcclient.NewWSClient(suite.Config.BftURI, "/websocket")
	go func() {
		// TODO: waiting for bft to accept websocket connection
		for {
			time.Sleep(1 * time.Second)
			err := bftClientWS.Start()
			if err != nil {
				fmt.Println("COULDNOT START THE BFTWS", err)
			} else {
				suite.BftSuite.BftRPCWSStatus = "up"
				break
			}
		}
	}()

	go func() {
		for {
			time.Sleep(1 * time.Second)
			if suite.BftSuite.BftRPCWSStatus != "up" {
				continue
			}
		}
		for e := range suite.BftSuite.BftRPCWS.ResponsesCh {
			queryString := gjson.GetBytes(e.Result, "query").String()
			if e.Error != nil {
				fmt.Println("BFTWS: websocket subscription received error:, ", e.Error.Error())
			}
			if suite.BftSuite.BftRPCWSQueryHandler.QueryMap[queryString] == nil {
				fmt.Println("BFTWS: websocket subscription received message but no listener, querystring", queryString)
				continue
			}
			suite.BftSuite.BftRPCWSQueryHandler.QueryMap[queryString] <- e.Result
			close(suite.BftSuite.BftRPCWSQueryHandler.QueryMap[queryString])
			delete(suite.BftSuite.BftRPCWSQueryHandler.QueryMap, queryString)
			ctx := context.Background()
			err := suite.BftSuite.BftRPCWS.Unsubscribe(ctx, queryString)
			if err != nil {
				fmt.Println("BFTWS: websocket could not unsubscribe, queryString", queryString)
			}
		}
	}()

	suite.BftSuite = &BftSuite{
		BftRPC:         &BftRPC{bftClient},
		BftRPCWS:       bftClientWS,
		BftRPCWSStatus: "down",
	}
}

type BftRPCWSQueryHandler struct {
	QueryMap map[string]chan []byte
}

func (bftSuite *BftSuite) RegisterQuery(query string) (chan []byte, error) {
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
	return responseCh, nil
}
