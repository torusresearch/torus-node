package keygen

import (
	"fmt"
	"math/big"
	"testing"

	"github.com/torusresearch/torus-public/secp256k1"

	"github.com/torusresearch/torus-public/common"
	"github.com/torusresearch/torus-public/pvss"
)

func TestKeygenDKG(test *testing.T) {
	n := 13
	k := 7
	t := 3
	var nodePrivKeys [13]big.Int
	var nodePubKeys [13]common.Point
	var nodeIndexes [13]int
	for i := 0; i < len(nodePrivKeys); i++ {
		nodePrivKeys[i] = *pvss.RandomBigInt()
		nodePubKeys[i] = common.BigIntToPoint(secp256k1.Curve.ScalarBaseMult(nodePrivKeys[i].Bytes()))
		nodeIndexes[i] = i + 2
	}
	nodeMapping := make(map[NodeDetailsID]NodeDetails)
	for i := 0; i < len(nodePrivKeys); i++ {
		nodeDetails := NodeDetails(common.Node{
			Index:  nodeIndexes[i],
			PubKey: nodePubKeys[i],
		})
		nodeMapping[nodeDetails.ToNodeDetailsID()] = nodeDetails
	}
	nodeNetwork := NodeNetwork{
		Nodes: nodeMapping,
		N:     n,
		T:     t,
		K:     k,
		ID:    "test-network",
	}

	fmt.Println(nodeNetwork)
}
