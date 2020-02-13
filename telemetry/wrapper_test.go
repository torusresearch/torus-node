package telemetry

import (
	"strconv"
	"testing"
)

func TestCreateAndRegister(t *testing.T) {

	// Create new metrics
	// Shouldn't throw any errors
	metrics := []CounterMetadata{
		{
			Name:   "test_counter_1",
			Help:   "",
			Prefix: "prefix_",
		},
		{
			Name:   "test_counter_2",
			Help:   "",
			Prefix: "prefix_",
		},
	}

	err := CreateAndRegister(metrics)

	if err != nil {
		t.Error("error while creating the metrics: " + err.Error())
		t.Fail()
	}

	// Add a new metric after the first initialisation
	// Shouldn't throw any errors
	metricsStage2 := []CounterMetadata{
		{
			Name:   "test_counter_3",
			Help:   "",
			Prefix: "prefix_",
		},
	}

	err = CreateAndRegister(metricsStage2)

	if err != nil {
		t.Error("error while creating the metrics: " + err.Error())
		t.Fail()
	}

	// Try to create a duplicate metric counter
	// Should throw an error
	metricsStage3 := []CounterMetadata{
		{
			Name:   "test_counter_1",
			Help:   "",
			Prefix: "prefix_",
		},
	}

	err = CreateAndRegister(metricsStage3)

	if err == nil {
		t.Error("expected error. Got null")
		t.Fail()
	}

	// Add a new metric with an invalid name
	// Should throw any errors
	metricsStage4 := []CounterMetadata{
		{
			Name:   "test_counter_!",
			Help:   "",
			Prefix: "prefix_",
		},
	}

	err = CreateAndRegister(metricsStage4)

	if err == nil {
		t.Error("expected error. Got nil.")
		t.Fail()
	}
}

func BenchmarkIncrementCounter(b *testing.B) {

	for i := 0; i < 10000; i++ {
		// Increment the counter
		IncrementCounter("test_counter_"+strconv.Itoa(i)+"_"+strconv.Itoa(b.N), "prefix_")
	}
}
