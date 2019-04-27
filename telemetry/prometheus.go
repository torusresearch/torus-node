package telemetry

import (
	"net/http"
	"sync"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// defaults
const (
	defaultCollectionPort = ":8080"
	defaultCollectionPath = "/metrics"
)

type Telemetry interface {
	Serve() error
	Register(metrics ...Metric) error
}

type Metric interface {
	collector() prometheus.Collector
}

type telemetry struct {
	sync.Mutex
	server *http.Server
}

// Register adds to the metrics collected.
func (m *telemetry) Register(metrics ...Metric) error {
	for _, metric := range metrics {
		err := prometheus.Register(metric.collector())
		if err != nil {
			return err
		}
	}
	return nil
}

// Serve will expose prometheus metrics.
func (m *telemetry) Serve() error {
	m.Lock()
	defer m.Unlock()
	m.server = &http.Server{Addr: defaultCollectionPort, Handler: promhttp.Handler()}
	return m.server.ListenAndServe()
}

var telemetryInstance *telemetry
var once sync.Once

func NewTelemetry() *telemetry {
	return &telemetry{}
}

func getTelemetry() *telemetry {
	once.Do(func() {
		telemetryInstance = NewTelemetry()
	})

	return telemetryInstance
}

// Counter is a cumulative metric that represents a single monotonically
// increasing counter whose value can only increase or be reset to zero on restart.
// For example, you can use a counter to represent the number of requests served,
// tasks completed, or errors.
// Do not use a counter to expose a value that can decrease. For example, do not
// use a counter for the number of currently running processes; instead use a
// gauge.
type Counter struct {
	name        string
	promCounter prometheus.Counter
}

func NewCounter(name, help string) *Counter {
	counter := prometheus.NewCounter(prometheus.CounterOpts{
		Name: name,
		Help: help,
	})
	return &Counter{
		name:        name,
		promCounter: counter,
	}
}

// Inc adds one to a given Counter.
func (c *Counter) Inc() {
	c.promCounter.Inc()
}

func (c *Counter) collector() prometheus.Collector {
	return c.promCounter
}

// Gauge is a metric that represents a single numerical value that can arbitrarily go up and down.
// Gauges are typically used for measured values like temperatures or current
// memory usage, but also "counts" that can go up and down, like the number of
// concurrent requests.
type Gauge struct {
	name      string
	promGauge prometheus.Gauge
}

func NewGauge(name, help string) *Gauge {
	gauge := prometheus.NewGauge(prometheus.GaugeOpts{
		Name: name,
		Help: help,
	})
	return &Gauge{
		name:      name,
		promGauge: gauge,
	}
}

func (g *Gauge) Set(val float64) {
	g.promGauge.Set(val)
}

func (g *Gauge) collector() prometheus.Collector {
	return g.promGauge
}

// Histogram samples observations (usually things like request durations or
// response sizes) and counts them in configurable buckets. It also provides a sum
// of all observed values.
// Histogram with a base metric name of <basename> exposes multiple time series during a scrape:
// - cumulative counters for the observation buckets, exposed as <basename>_bucket{le="<upper inclusive bound>"}
// - the total sum of all observed values, exposed as <basename>_sum
// - the count of events that have been observed, exposed as <basename>_count (identical to <basename>_bucket{le="+Inf"} above)
type Histogram struct {
	name          string
	promHistogram prometheus.Histogram
}

func (h *Histogram) Observe(val float64) {
	h.promHistogram.Observe(val)
}

func (h *Histogram) collector() prometheus.Collector {
	return h.promHistogram
}

// Summary, similarly to a histogram, samples observations (usually things like
// request durations and response sizes). While it also provides a total count of
// observations and a sum of all observed values, it calculates configurable
// quantiles over a sliding time window.
// A summary with a base metric name of <basename> exposes multiple time series during a scrape:
// - streaming φ-quantiles (0 ≤ φ ≤ 1) of observed events, exposed as <basename>{quantile="<φ>"}
// - the total sum of all observed values, exposed as <basename>_sum
// - the count of events that have been observed, exposed as <basename>_count
type Summary struct {
	name        string
	promSummary prometheus.Summary
}

func (s *Summary) Observe(val float64) {
	s.promSummary.Observe(val)
}

func (s *Summary) collector() prometheus.Collector {
	return s.promSummary
}

// Serve will start an http server exposing the "/metrics" endpoint for the default telemetry.
func Serve() error {
	return getTelemetry().Serve()
}

// Register registers the metrics to be collected for the default telemetry.
func Register(metrics ...Metric) error {
	return getTelemetry().Register(metrics...)
}
