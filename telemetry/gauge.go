package telemetry

import (
	"regexp"
	"sync"

	logging "github.com/sirupsen/logrus"
)

var DebugGauges = &DebugGaugesSyncMap{}

type DebugGaugesSyncMap struct {
	sync.Map
}

func (m *DebugGaugesSyncMap) GetOrSet(gaugeName string, gauge *Gauge) (res *Gauge, found bool) {
	resInter, found := m.Map.LoadOrStore(gaugeName, gauge)
	res, _ = resInter.(*Gauge)
	return
}

func IncGauge(name string) {
	go incGauge(name)
}

func DecGauge(name string) {
	go decGauge(name)
}

func incGauge(name string) {

	reg, err := regexp.Compile("[^a-zA-Z0-9]+")
	if err != nil {
		logging.WithField("telemetry: incGauge", name).WithError(err).Error("regEx error")
		return
	}

	gaugeName := reg.ReplaceAllString(name, "")
	gauge, found := DebugGauges.GetOrSet(gaugeName, NewGauge("gauge_"+gaugeName, "number of "+gaugeName))
	if !found {
		err := Register(gauge)
		if err != nil {
			logging.WithField("telemetry: incGauge", gaugeName).WithError(err).Error("error while registering the gauge")
		}
	}
	gauge.Inc()
}

func decGauge(name string) {

	reg, err := regexp.Compile("[^a-zA-Z0-9]+")
	if err != nil {
		logging.WithField("telemetry: decGauge", name).WithError(err).Error("regEx error")
		logging.Fatal(err)
	}
	gaugeName := reg.ReplaceAllString(name, "")
	gauge, found := DebugGauges.GetOrSet(gaugeName, NewGauge("gauge_"+gaugeName, "number of "+gaugeName))
	if !found {
		err := Register(gauge)
		if err != nil {
			logging.WithField("telemetry: decGauge", gaugeName).WithError(err).Error("error while registering the gauge")
		}
	}
	gauge.Dec()
}
