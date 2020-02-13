package msgqueue

import (
	"time"

	logging "github.com/sirupsen/logrus"

	"github.com/avast/retry-go"
)

type MessageQueue struct {
	queue          chan queuedMsg
	processMessage func(msg []byte) (response interface{}, err error)
}

type queuedMsg struct {
	Response chan interface{}
	Msg      []byte
}

func NewMessageQueue(processMessageFunc func(msg []byte) (response interface{}, err error)) *MessageQueue {
	return &MessageQueue{
		queue:          make(chan queuedMsg),
		processMessage: processMessageFunc,
	}
}

func (q *MessageQueue) Add(bftTxBytes []byte) (res interface{}) {
	resChan := make(chan interface{})
	q.queue <- queuedMsg{resChan, bftTxBytes}
	return <-resChan
}

func (q *MessageQueue) RunMsgEngine(num int) {
	for i := 0; i < num; i++ {
		go func() {
			for {
				bftMsg := <-q.queue
				var response interface{}
				var err error
				_ = retry.Do(func() error {
					response, err = q.processMessage(bftMsg.Msg)
					if err != nil {
						return err
					}
					return nil
				},
					retry.Attempts(600),
					retry.Delay(1*time.Second),
					retry.DelayType(retry.FixedDelay),
				)
				if err != nil {
					logging.WithError(err).WithField("bftMsg", bftMsg).Error("could not process message in run msg engine")
				}
				bftMsg.Response <- response
			}
		}()
	}
}
