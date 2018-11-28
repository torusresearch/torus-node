package dkgnode

import (
	"fmt"
	"time"

	"github.com/tendermint/tendermint/rpc/client"
	rpcclient "github.com/tendermint/tendermint/rpc/lib/client"
)

type BftSuite struct {
	BftRPC   *BftRPC
	BftRPCWS *rpcclient.WSClient
}

type BftRPCWS struct {
	*rpcclient.WSClient
}

func SetUpBft(suite *Suite) {
	//TODO: keep server connection around for logging??
	//commented out for testing purposes
	// go tmabci.RunABCIServer()
	// var wg sync.WaitGroup
	// wg.Add(1)

	bftClient := client.NewHTTP(suite.Config.BftURI, "/websocket")
	// fmt.Println("bftclientwseventsgetws,", bftClient.WSEvents.GetWS())
	// bftClient.WSEvents.GetWS().SetCodec(bftClient.GetCodec())
	bftClientWS := rpcclient.NewWSClient(suite.Config.BftURI, "/websocket")
	go func() {
		// TODO: waiting for bft to accept websocket connection
		time.Sleep(time.Second * 10)
		err := bftClientWS.Start()
		if err != nil {
			fmt.Println("COULDNOT START THE BFTWS", err)
		}
		// wg.Done()
	}()
	suite.BftSuite = &BftSuite{
		BftRPC:   &BftRPC{bftClient},
		BftRPCWS: bftClientWS,
	}
	// wg.Wait()
}
