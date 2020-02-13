package msgqueue

import (
	"fmt"
	"os"
	"strconv"
	"testing"
	"time"
)

func TestMessageQueue(t *testing.T) {
	testResp := "oooweweee"
	q := NewMessageQueue(func(msg []byte) (interface{}, error) {
		time.Sleep(1 * time.Second)
		return fmt.Sprintf("%s %v", testResp, string(msg)), nil
	})
	q.RunMsgEngine(10)

	// testing for concurrency
	end := make(chan string)
	numCom := 20
	for i := 0; i < numCom; i++ {
		go func(num int) {
			res := q.Add([]byte(strconv.Itoa(num)))
			if res.(string) != fmt.Sprintf("%s %v", testResp, strconv.Itoa(num)) {
				fmt.Println("failed")
				os.Exit(0)
			}
			end <- ""
		}(i)
	}
	countTwo := 0
	for countTwo < numCom {
		<-end
		countTwo++
	}
}
