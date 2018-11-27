package dkgnode

import "github.com/tendermint/tendermint/rpc/client"

type BftSuite struct {
	BftRPC *BftRPC
}

func SetUpBftRPC(suite *Suite) {
	suite.BftSuite = &BftSuite{BftRPC: NewBftRPC(suite.Config.BftURI)}
}

func NewBftRPC(uri string) *BftRPC {
	//TODO: keep server connection around for logging??
	//commented out for testing purposes
	// go tmabci.RunABCIServer()

	bftClient := client.NewHTTP(uri, "/websocket")
	return &BftRPC{
		bftClient,
	}
}
