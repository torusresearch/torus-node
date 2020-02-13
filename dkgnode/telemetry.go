package dkgnode

import (
	"context"

	logging "github.com/sirupsen/logrus"
	"github.com/torusresearch/torus-node/eventbus"
	"github.com/torusresearch/torus-node/telemetry"
)

var Events = struct {
	ErrorEvent string
	StartEvent string
	StopEvent  string
}{
	ErrorEvent: "error",
	StartEvent: "start",
	StopEvent:  "stop",
}

type TelemetryService struct {
	bs             *BaseService
	cancel         context.CancelFunc
	ctx            context.Context
	eventBus       eventbus.Bus
	serviceLibrary ServiceLibrary
}

func (t *TelemetryService) Name() string {
	return "telemetry"
}

func (t *TelemetryService) OnStart() error {
	go func() {
		err := telemetry.Serve(t.ctx)
		if err != nil {
			logging.WithError(err).Error("could not start telemetry")
			t.eventBus.Publish("telemetryService:"+Events.ErrorEvent, err)
		}
	}()
	t.eventBus.Publish("telemetryService:"+Events.StartEvent, nil)
	return nil
}

func (t *TelemetryService) OnStop() error {
	t.cancel()
	t.eventBus.Publish("telemetry_service:"+Events.StopEvent, nil)
	return nil
}

func (t *TelemetryService) Call(method string, args ...interface{}) (interface{}, error) {
	return nil, nil
}

func (t *TelemetryService) SetBaseService(bs *BaseService) {
	t.bs = bs
}

func NewTelemetryService(ctx context.Context, eventBus eventbus.Bus) *BaseService {
	telemetryCtx, cancel := context.WithCancel(context.WithValue(ctx, ContextID, "telemetry"))
	telemetryService := TelemetryService{
		cancel:   cancel,
		ctx:      telemetryCtx,
		eventBus: eventBus,
	}
	telemetryService.serviceLibrary = NewServiceLibrary(telemetryService.eventBus, telemetryService.Name())
	return NewBaseService(&telemetryService)
}
