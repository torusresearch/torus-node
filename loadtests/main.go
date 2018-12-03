package main

import (
	"fmt"
	"math/rand"
	"net/http"
	"strconv"
	"time"

	vegeta "github.com/tsenart/vegeta/lib"
)

func main() {
	rate := vegeta.Rate{Freq: 5, Per: time.Second}
	duration := 4 * time.Second
	targeter := NewCustomTargeter()
	attacker := vegeta.NewAttacker()

	var metrics vegeta.Metrics
	for res := range attacker.Attack(targeter, rate, duration, "Big Bang!") {
		metrics.Add(res)
		fmt.Println("response: ", string(res.Body))
	}

	metrics.Close()

	fmt.Printf("99th percentile: %s\n", metrics.Latencies.P99)
	fmt.Println("requests: ", metrics.Requests)
	fmt.Println("statuscodes: ", metrics.StatusCodes)
}

func NewCustomTargeter() vegeta.Targeter {
	return func(tgt *vegeta.Target) error {
		if tgt == nil {
			return vegeta.ErrNilTarget
		}

		tgt.Method = "POST"
		tgt.URL = "http://localhost:8001/jrpc"
		rand := rand.Int()
		tgt.Header = http.Header{"Content-Type": []string{"application/json"}}

		payload := `{"jsonrpc": "2.0","method": "SecretAssign","id": 6,"params": {"email": "` + strconv.Itoa(rand) + `@gmail.com"}}`
		tgt.Body = []byte(payload)
		fmt.Println("Sent: " + payload)
		return nil
	}
}
