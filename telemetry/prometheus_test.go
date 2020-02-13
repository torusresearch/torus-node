package telemetry

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestServe(t *testing.T) {
	c := NewCounter("test_counter", "Testing the counter")
	telemetry := NewTelemetry()
	_ = telemetry.Register(c)
	incTimes := 5

	for i := 0; i < incTimes; i++ {
		c.Inc()
	}

	done := make(chan struct{})
	go func(d chan struct{}) {
		go func() {
			_ = telemetry.Serve()
		}()

		<-d
		telemetry.server.Close()

	}(done)
	time.Sleep(5 * time.Second)

	resp, err := http.Get("http://localhost:8080/metrics")
	if err != nil {
		t.Log(err)
	}

	if resp.StatusCode != 200 {
		t.Logf("Wrong status code. Expected: 200, got: %d", resp.StatusCode)
		t.Fail()
	}
	close(done)

	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		t.Log(err)
		t.Fail()
	}

	expected := fmt.Sprintf("test_counter %d", incTimes)
	if !strings.Contains(string(body), expected) {
		t.Log("Response:", string(body))
		t.Logf("Response did not contain expected metric: %s", expected)
		t.Fail()
	}
}
