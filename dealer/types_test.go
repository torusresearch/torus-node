package dealer

import (
	"fmt"
	"math/big"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/torusresearch/bijson"
	"github.com/torusresearch/torus-common/common"
	pcmn "github.com/torusresearch/torus-node/common"
)

func TestBytes(t *testing.T) {
	fmt.Println("Hello, playground")
	var d MsgUpdateCommitment
	d.KeyIndex = *big.NewInt(1)
	d.TMessage = TMessage{
		Timestamp: strconv.Itoa(int(time.Now().Unix())),
		Message:   strings.Join([]string{"mug00", "updateShare"}, pcmn.Delimiter1),
	}
	d.Commitment = [][]common.Point{[]common.Point{common.Point{X: *big.NewInt(0), Y: *big.NewInt(0)}}}

	byt, _ := bijson.Marshal(d)
	// var dm DealerMessageParams
	fmt.Println(string(byt))
	dealerMessage := Message{
		Method:    "updateCommitment",
		KeyIndex:  d.KeyIndex,
		PubKey:    common.Point{},
		Data:      byt,
		Signature: []byte{},
	}
	bytes, _ := bijson.Marshal(dealerMessage)
	fmt.Println(string(bytes))
	t.Fail()
}
