package dkgnode

import (
	"github.com/YZhenY/DKGNode/bft"
	jsonrpcclient "github.com/ybbus/jsonrpc"
)

type BftRPC struct {
	jsonrpcclient.RPCClient
}

func NewBftRPC(uri string) *BftRPC {
	rpcClient := jsonrpcclient.NewClient(uri)
	return &BftRPC{
		RPCClient: rpcClient,
	}
}

func (bftrpc BftRPC) Epoch() (int, error) {
	res, err := bftrpc.Call("Epoch", bft.EpochParams{})
	if err != nil {
		return 0, err
	}
	var obj bft.EpochResult
	if err := res.GetObject(&obj); err != nil {
		return 0, err
	}
	return obj.Epoch, nil
}

func (bftrpc BftRPC) SetEpoch(epoch int) (int, error) {
	res, err := bftrpc.Call("Epoch", bft.SetEpochParams{Epoch: epoch})
	if err != nil {
		return 0, err
	}
	var obj bft.SetEpochResult
	if err := res.GetObject(&obj); err != nil {
		return 0, err
	}
	return obj.Epoch, nil
}

func (bftrpc BftRPC) Broadcast(data []byte) (int, error) {
	stringData := string(data[:])
	res, err := bftrpc.Call("Broadcast", bft.BroadcastParams{Data: stringData, Length: len(stringData)})
	if err != nil {
		return 0, err
	}
	var obj bft.BroadcastResult
	if err := res.GetObject(&obj); err != nil {
		return 0, err
	}
	return obj.Id, nil
}

func (bftrpc BftRPC) Retrieve(id int) (data []byte, length int, err error) {
	res, err := bftrpc.Call("Retrieve", bft.RetrieveParams{Id: id})
	if err != nil {
		return nil, 0, err
	}
	var obj bft.RetrieveResult
	if err := res.GetObject(&obj); err != nil {
		return nil, 0, err
	}
	data = []byte(obj.Data)
	length = obj.Length
	return
}
