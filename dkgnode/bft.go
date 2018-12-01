package dkgnode

import (
	"fmt"
	"time"

	"github.com/tendermint/tendermint/node"
	"github.com/tendermint/tendermint/rpc/client"
	rpcclient "github.com/tendermint/tendermint/rpc/lib/client"
)

type BftSuite struct {
	BftRPC    *BftRPC
	BftRPCWS  *rpcclient.WSClient
	BftNode   *node.Node
	UpdateVal bool
}

type BftRPCWS struct {
	*rpcclient.WSClient
}

func SetUpBft(suite *Suite) {
	// TODO: keep server connection around for logging??
	// commented out for testing purposes
	// go tmabci.RunABCIServer()

	bftClient := client.NewHTTP(suite.Config.BftURI, "/websocket")

	// for subscribe and unsubscribe method calls, use this
	bftClientWS := rpcclient.NewWSClient(suite.Config.BftURI, "/websocket")
	go func() {
		// TODO: waiting for bft to accept websocket connection
		time.Sleep(time.Second * 30)
		err := bftClientWS.Start()
		if err != nil {
			fmt.Println("COULDNOT START THE BFTWS", err)
		}
	}()
	suite.BftSuite = &BftSuite{
		BftRPC:   &BftRPC{bftClient},
		BftRPCWS: bftClientWS,
	}
}
