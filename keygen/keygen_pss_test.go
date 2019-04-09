package keygen

import (
	"fmt"
	"math/big"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/torusresearch/torus-public/secp256k1"

	"github.com/torusresearch/torus-public/common"
	"github.com/torusresearch/torus-public/pvss"
)

func TestPMMarshal(test *testing.T) {
	secret := pvss.RandomBigInt()
	mask := pvss.RandomBigInt()
	f := pvss.GenerateRandomBivariatePolynomial(*secret, 13)
	fprime := pvss.GenerateRandomBivariatePolynomial(*mask, 13)
	C := pvss.GetCommitmentMatrix(f, fprime)
	data := BaseParser.MarshalPM(C)
	Creconstructed := BaseParser.UnmarshalPM(data)
	for i := 0; i < 13; i++ {
		for j := 0; j < 13; j++ {
			assert.Equal(test, C[i][j].X.Text(16), Creconstructed[i][j].X.Text(16))
		}
	}
}

func SetupTestNodes() ([]*PSSNode, []common.Node, int, int, int) {
	// setup
	n := 13
	k := 7
	t := 3
	var nodePrivKeys []big.Int
	var nodePubKeys []common.Point
	var nodeIndexes []int
	for i := 0; i < n; i++ {
		nodePrivKeys = append(nodePrivKeys, *pvss.RandomBigInt())
		nodePubKeys = append(nodePubKeys, common.BigIntToPoint(secp256k1.Curve.ScalarBaseMult(nodePrivKeys[i].Bytes())))
		nodeIndexes = append(nodeIndexes, i+1)
	}
	var nodeList []common.Node
	for i := 0; i < n; i++ {
		nodeList = append(nodeList, common.Node{
			Index:  nodeIndexes[i],
			PubKey: nodePubKeys[i],
		})
	}
	localTransportNodeDirectory := make(map[NodeDetailsID]*LocalTransport)
	var nodes []*PSSNode
	for i := 0; i < n; i++ {
		node := common.Node{
			Index:  nodeIndexes[i],
			PubKey: nodePubKeys[i],
		}
		localTransport := LocalTransport{
			NodeDirectory: &localTransportNodeDirectory,
		}
		newPssNode := NewPSSNode(
			node,
			nodeList,
			t,
			k,
			nodeList,
			t,
			k,
			*big.NewInt(int64(i + 1)),
			&localTransport,
		)
		nodes = append(nodes, newPssNode)
		localTransport.SetPSSNode(newPssNode)
		nodeDetails := NodeDetails(node)
		localTransportNodeDirectory[nodeDetails.ToNodeDetailsID()] = &localTransport
	}
	return nodes, nodeList, n, k, t
}

func TestKeygenSharing(test *testing.T) {
	nodes, nodeList, n, k, t := SetupTestNodes()
	fmt.Println("Running TestKeygenSharing for " + strconv.Itoa(n) + " nodes with reconstruction " + strconv.Itoa(k) + " and threshold " + strconv.Itoa(t))
	originalPrivateKey := pvss.RandomBigInt()
	mask := pvss.RandomBigInt()
	randPoly := pvss.RandomPoly(*originalPrivateKey, k)
	randPolyprime := pvss.RandomPoly(*mask, k)
	Si := *pvss.PolyEval(*randPoly, *big.NewInt(int64(1)))
	Siprime := *pvss.PolyEval(*randPolyprime, *big.NewInt(int64(1)))
	// initiate keygen
	nodes[0].ShareStore[SharingID("TestKeygenSharing")] = &Sharing{
		SharingID: SharingID("TestKeygenSharing"),
		Nodes:     nodeList,
		Epoch:     1,
		I:         1,
		Si:        Si,
		Siprime:   Siprime,
	}
	pssMsgShare := PSSMsgShare{
		SharingID: SharingID("TestKeygenSharing"),
	}
	err := nodes[0].Transport.Send(nodes[0].NodeDetails, PSSMessage{
		PSSID: (&PSSIDDetails{
			SharingID: SharingID("TestKeygenSharing"),
			Index:     1,
		}).ToPSSID(),
		Method: "share",
		Data:   pssMsgShare.ToBytes(),
	})
	assert.NoError(test, err)
	select {}
}
