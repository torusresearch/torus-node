package telemetry

import (
	"errors"
	"regexp"
	"sync"

	logging "github.com/sirupsen/logrus"
)

// counters ...
// Holds the reference to the registered counters
var counters sync.Map

// CounterMetadata ...
// Holds the metadata of the counters
// Used during registration as middleware between the services and the prometheus
type CounterMetadata struct {
	Prefix string `mapstructure:"prefix"`
	Name   string `mapstructure:"name"`
	Help   string `mapstructure:"help"`
}

// Configuration ...
// A structure to read the config into
type Configuration struct {
	Metrics map[string]CounterMetadata `mapstructure:"metrics"`
}

// CreateAndRegister Creates a metric and registers it
func CreateAndRegister(countersMetadata []CounterMetadata) error {

	var (
		helpText   string
		errWrapper error // Wrap the errors occurred during the process
	)

	for i := range countersMetadata {

		// Validate the name before creating
		reg := regexp.MustCompile(`^[a-zA-Z0-9_]`)

		if !reg.MatchString(countersMetadata[i].Name) {
			if errWrapper == nil {
				errWrapper = errors.New("failed to create metric for " + countersMetadata[i].Name + " reason: invalid name... ")
			} else {
				errWrapper = errors.New(errWrapper.Error() + "failed to create metric for " + countersMetadata[i].Name + " reason: invalid name.... ")
			}

			continue
		}

		// Check if the counter is already registered
		_, ok := counters.Load(countersMetadata[i].Prefix + countersMetadata[i].Name)

		if ok {
			logging.WithField("telemetry: CreateAndRegister", countersMetadata[i].Name).WithError(errors.New("counter already exists")).Error("could not register")

			if errWrapper == nil {

				errWrapper = errors.New("failed to create metric for " + countersMetadata[i].Name + " reason: it already exists... ")
			} else {

				errWrapper = errors.New(errWrapper.Error() + "failed to create metric for " + countersMetadata[i].Name + " reason: it already exists.... ")
			}

			continue
		}

		// Check if the help text is provided
		// If not, add a default one
		if countersMetadata[i].Help == "" {
			helpText = "number of " + countersMetadata[i].Name
		} else {
			helpText = countersMetadata[i].Help
		}

		// Create a counter using the specified metadata
		counter := NewCounter(countersMetadata[i].Prefix+countersMetadata[i].Name, helpText)

		// Register the counter with prometheus
		err := Register(counter)
		if err != nil {
			logging.WithField("telemetry: CreateAndRegister", countersMetadata[i].Name).WithError(err).Error("could not register")

			if errWrapper == nil {
				errWrapper = errors.New("failed to create metric for " + countersMetadata[i].Name + " reason: " + err.Error() + "... ")
			} else {
				errWrapper = errors.New(errWrapper.Error() + "failed to create metric for " + countersMetadata[i].Name + " reason: " + err.Error() + ".... ")
			}

			continue
		}

		// Store the reference to the counter in the in-memory store
		counters.Store(countersMetadata[i].Prefix+countersMetadata[i].Name, counter)
	}

	return errWrapper
}

// IncrementCounter ...
// A public method which increments the counter for the specified counter name
func IncrementCounter(metricName, prefix string) {

	go incrementCounter(metricName, prefix)
}

func incrementCounter(metricName, prefix string) {

	name := prefix + metricName

	// Get the reference to the counter
	_, ok := counters.Load(name)

	if !ok {
		// Counter doesn't exist. Create and then increment
		logging.WithField("telemetry: IncrementCounter", name).Debug("could not find the counter. Creating ... ")

		counterMetadata := CounterMetadata{
			Prefix: prefix,
			Name:   metricName,
			Help:   "",
		}

		err := CreateAndRegister([]CounterMetadata{counterMetadata})

		if err != nil {
			logging.WithField("telemetry: IncrementCounter", name).WithError(err).Errorln("could not create the metric")
		}
	}

	value, _ := counters.Load(name)
	// Increment the counter
	counter, ok := value.(*Counter)

	if !ok {
		logging.WithField("telemetry: IncrementCounter", name).Errorln("error while casting interface{} to counter")
	} else {
		counter.Inc()
	}
}
