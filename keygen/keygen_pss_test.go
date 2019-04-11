package keygen

import (
	"fmt"
	"math/big"
	"runtime"
	"strconv"
	"strings"
	"testing"

	"github.com/torusresearch/torus-public/logging"

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

func SetupTestNodes(n, k, t int) (chan string, []*PSSNode, []common.Node) {
	engineState := make(map[string]interface{})

	runEngine := func(nodeDetails NodeDetails, pssMessage PSSMessage) error {
		if _, found := engineState["test"]; found {
			fmt.Println("test found")
		}
		fmt.Println("MockTMEngine - Node:", nodeDetails.Index, " pssMessage:", pssMessage)
		return nil
	}
	// setup
	commCh := make(chan string)
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
			OutputChannel: &commCh,
			MockTMEngine:  &runEngine,
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
	return commCh, nodes, nodeList
}

func TestKeygenSharing(test *testing.T) {
	logging.SetLevelString("error")
	runtime.GOMAXPROCS(10)
	keys := 1
	n := 9
	k := 5
	t := 2
	commCh, nodes, nodeList := SetupTestNodes(n, k, t)
	fmt.Println("Running TestKeygenSharing for " + strconv.Itoa(keys) + " keys, " + strconv.Itoa(n) + " nodes with reconstruction " + strconv.Itoa(k) + " and threshold " + strconv.Itoa(t))
	var secrets []big.Int
	var sharingIDs []SharingID
	for h := 0; h < keys; h++ {
		secret := pvss.RandomBigInt()
		secrets = append(secrets, *secret)
		mask := pvss.RandomBigInt()
		randPoly := pvss.RandomPoly(*secret, k)
		randPolyprime := pvss.RandomPoly(*mask, k)
		commit := pvss.GetCommit(*randPoly)
		commitH := pvss.GetCommitH(*randPolyprime)
		sumCommitments := pvss.AddCommitments(commit, commitH)
		sharingID := SharingID("SHARING" + strconv.Itoa(h))
		sharingIDs = append(sharingIDs, sharingID)
		for _, node := range nodes {
			index := node.NodeDetails.Index
			Si := *pvss.PolyEval(*randPoly, *big.NewInt(int64(index)))
			Siprime := *pvss.PolyEval(*randPolyprime, *big.NewInt(int64(index)))
			node.ShareStore[sharingID] = &Sharing{
				SharingID: sharingID,
				Nodes:     nodeList,
				Epoch:     1,
				I:         index,
				Si:        Si,
				Siprime:   Siprime,
				C:         sumCommitments,
			}
		}
	}
	for _, sharingID := range sharingIDs {
		for _, node := range nodes {
			pssMsgShare := PSSMsgShare{
				SharingID: sharingID,
			}
			pssID := (&PSSIDDetails{
				SharingID: sharingID,
				Index:     node.NodeDetails.Index,
			}).ToPSSID()
			err := node.Transport.Send(node.NodeDetails, PSSMessage{
				PSSID:  pssID,
				Method: "share",
				Data:   pssMsgShare.ToBytes(),
			})
			assert.NoError(test, err)
		}
	}
	completeMessages := 0
	for completeMessages < n*n*keys {
		msg := <-commCh
		if strings.Contains(msg, "shared") {
			completeMessages++
		} else {
			assert.Fail(test, "did not get the required number of share complete messages")
		}
	}
	assert.Equal(test, completeMessages, n*n*keys)
	for g, sharingID := range sharingIDs {
		var shares []common.PrimaryShare
		for _, node := range nodes {
			var subshares []common.PrimaryShare
			for _, noderef := range nodes { // assuming that all nodes are part of the valid set
				val := node.PSSStore[(&PSSIDDetails{
					SharingID: sharingID,
					Index:     noderef.NodeDetails.Index,
				}).ToPSSID()].Si
				if val.Cmp(big.NewInt(int64(0))) != 0 {
					subshares = append(subshares, common.PrimaryShare{
						Index: noderef.NodeDetails.Index,
						Value: val,
					})
				}
			}
			reconstructedSi := pvss.LagrangeScalar(subshares, 0)
			shares = append(shares, common.PrimaryShare{
				Index: node.NodeDetails.Index,
				Value: *reconstructedSi,
			})
		}
		reconstructedSecret := pvss.LagrangeScalar(shares, 0)
		assert.Equal(test, reconstructedSecret.Text(16), secrets[g].Text(16))
	}
}
