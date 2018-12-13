package main

import (
	"fmt"
	"math/rand"
	"net/http"
	"strconv"
	"time"

	vegeta "github.com/tsenart/vegeta/lib"
)

var mostRecent = "avgjoe@gmail.com"
var ratio = 5 //20% assignments

var NodeAddressesList = []string{
	"node1.torusnode.com",
	"node2.torusnode.com",
	"node3.torusnode.com",
	"node4.torusnode.com",
	"node5.torusnode.com",
}

//for metrics and data
var assignments = 0
var sharerequests = 0

func main() {
	// rate := vegeta.Rate{Freq: 3, Per: time.Second}
	duration := 4 * time.Second
	targeter := NewCustomTargeter()
	attacker := vegeta.NewAttacker()

	var metrics vegeta.Metrics
	for res := range attacker.Attack(targeter, uint64(100), duration) {
		metrics.Add(res)

		// fmt.Println("response: ", res.Timestamp, strconv.FormatInt(int64(res.BytesIn), 16))
	}

	metrics.Close()
	fmt.Printf("Mean: %s\n", metrics.Latencies.Mean)
	fmt.Printf("95th percentile: %s\n", metrics.Latencies.P95)
	fmt.Println("requests: ", metrics.Requests)
	fmt.Println("statuscodes: ", metrics.StatusCodes)
	fmt.Println("assignmnets: ", assignments)
	fmt.Println("sharerequests: ", sharerequests)
}

func NewCustomTargeter() vegeta.Targeter {
	return func(tgt *vegeta.Target) error {
		if tgt == nil {
			return vegeta.ErrNilTarget
		}
		rand := rand.Int() + int(time.Now().Unix())
		randmod := rand % 4
		randMod2 := rand % ratio
		tgt.Method = "POST"
		tgt.URL = "https://" + NodeAddressesList[randmod] + "/jrpc"
		tgt.Header = http.Header{"Content-Type": []string{"application/json"}}
		var payload string
		if randMod2 < ratio-1 {
			payload = `{"jsonrpc": "2.0","method": "ShareRequest","id": 6,"params": {"index": 0,"email": "s3asfdsf@gmail.com","idtoken": "blublu"}}`
			tgt.Body = []byte(payload)
			sharerequests = sharerequests + 1
		} else {
			payload = `{"jsonrpc": "2.0","method": "SecretAssign","id": 6,"params": {"email": "` + strconv.Itoa(rand) + `@gmail.com"}}`
			tgt.Body = []byte(payload)
			assignments = assignments + 1
		}
		// fmt.Println("Sent: " + payload)

		return nil
	}
}
