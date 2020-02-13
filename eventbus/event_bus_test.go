package eventbus

import (
	"testing"
	"time"
)

func TestNew(t *testing.T) {
	bus := New()
	if bus == nil {
		t.Log("New EventBus not created!")
		t.Fail()
	}
}

func TestHasCallback(t *testing.T) {
	bus := New()
	_ = bus.Subscribe("topic", func(interface{}) {})
	if bus.HasCallback("topic_topic") {
		t.Fail()
	}
	if !bus.HasCallback("topic") {
		t.Fail()
	}
}

func TestSubscribe(t *testing.T) {
	bus := New()
	if bus.Subscribe("topic", func(interface{}) {}) != nil {
		t.Fail()
	}
}

func TestSubscribeOnce(t *testing.T) {
	bus := New()
	if bus.SubscribeOnce("topic", func(interface{}) {}) != nil {
		t.Fail()
	}
}

func TestSubscribeOnceAndManySubscribe(t *testing.T) {
	bus := New()
	event := "topic"
	flag := 0
	fn := func(interface{}) { flag += 1 }
	_ = bus.SubscribeOnce(event, fn)
	_ = bus.Subscribe(event, fn)
	_ = bus.Subscribe(event, fn)
	bus.Publish(event, nil)

	if flag != 3 {
		t.Fail()
	}
}

func TestUnsubscribe(t *testing.T) {
	bus := New()
	handler := func(interface{}) {}
	_ = bus.Subscribe("topic", handler)
	if bus.Unsubscribe("topic", handler) != nil {
		t.Fail()
	}
	if bus.Unsubscribe("topic", handler) == nil {
		t.Fail()
	}
}

func TestUnsubscribeAll(t *testing.T) {
	bus := New()
	handler := func(interface{}) {}
	_ = bus.Subscribe("topic", handler)
	if bus.UnsubscribeAll("topic") != nil {
		t.Fail()
	}
	if bus.UnsubscribeAll("topic") == nil {
		t.Fail()
	}
}

type AB struct {
	A int
	B int
}

func TestPublish(t *testing.T) {
	bus := New()
	_ = bus.Subscribe("topic", func(ab interface{}) {
		abstruct := ab.(AB)
		if abstruct.A != abstruct.B {
			t.Fail()
		}
	})
	bus.Publish("topic", AB{A: 1, B: 1})
}

type AOUT struct {
	A   int
	Out *[]int
}

func TestSubcribeOnceAsync(t *testing.T) {
	results := make([]int, 0)

	bus := New()
	_ = bus.SubscribeOnceAsync("topic", func(aout interface{}) {
		aoutstruct := aout.(AOUT)
		*aoutstruct.Out = append(*aoutstruct.Out, aoutstruct.A)
	})

	bus.Publish("topic", AOUT{A: 10, Out: &results})
	bus.Publish("topic", AOUT{A: 10, Out: &results})

	bus.WaitAsync()

	if len(results) != 1 {
		t.Fail()
	}

	if bus.HasCallback("topic") {
		t.Fail()
	}
}

type AOUTDUR struct {
	A   int
	Out *[]int
	Dur string
}

func TestSubscribeAsyncTransactional(t *testing.T) {
	results := make([]int, 0)

	bus := New()
	_ = bus.SubscribeAsync("topic", func(aoutdur interface{}) {
		aoutdurstruct := aoutdur.(AOUTDUR)
		sleep, _ := time.ParseDuration(aoutdurstruct.Dur)
		time.Sleep(sleep)
		*aoutdurstruct.Out = append(*aoutdurstruct.Out, aoutdurstruct.A)
	}, true)

	bus.Publish("topic", AOUTDUR{1, &results, "1s"})
	bus.Publish("topic", AOUTDUR{2, &results, "0s"})

	bus.WaitAsync()

	if len(results) != 2 {
		t.Fail()
	}

	if results[0] != 1 || results[1] != 2 {
		t.Fail()
	}
}

type AOUTCH struct {
	A   int
	Out chan<- int
}

func TestSubscribeAsync(t *testing.T) {
	results := make(chan int)

	bus := New()
	_ = bus.SubscribeAsync("topic", func(aoutch interface{}) {
		aoutchstruct := aoutch.(AOUTCH)
		aoutchstruct.Out <- aoutchstruct.A
	}, false)

	bus.Publish("topic", AOUTCH{1, results})
	bus.Publish("topic", AOUTCH{2, results})

	numResults := 0

	go func() {
		for range results {
			numResults++
		}
	}()

	bus.WaitAsync()

	time.Sleep(10 * time.Millisecond)

	if numResults != 2 {
		t.Fail()
	}
}
