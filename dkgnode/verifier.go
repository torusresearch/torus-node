package dkgnode

import (
	"context"
	"fmt"

	pcmn "github.com/torusresearch/torus-node/common"
	"github.com/torusresearch/torus-node/telemetry"

	"github.com/torusresearch/bijson"
	"github.com/torusresearch/torus-node/auth"
	"github.com/torusresearch/torus-node/config"
	"github.com/torusresearch/torus-node/eventbus"
)

func NewVerifierService(ctx context.Context, eventBus eventbus.Bus) *BaseService {
	verifierCtx, cancel := context.WithCancel(context.WithValue(ctx, ContextID, "verifier"))
	verifierService := VerifierService{
		cancel:        cancel,
		parentContext: ctx,
		context:       verifierCtx,
		eventBus:      eventBus,
	}
	verifierService.serviceLibrary = NewServiceLibrary(verifierService.eventBus, verifierService.Name())
	return NewBaseService(&verifierService)
}

type VerifierService struct {
	defaultVerifier auth.GeneralVerifier

	bs             *BaseService
	cancel         context.CancelFunc
	parentContext  context.Context
	context        context.Context
	eventBus       eventbus.Bus
	serviceLibrary ServiceLibrary
}

func (v *VerifierService) OnStart() error {
	// We can use a flag here to change the default verifier
	// In the future we should allow a range of verifiers
	verifiers := []auth.Verifier{
		auth.NewGoogleVerifier(),
		auth.NewDiscordVerifier(),
		auth.NewFacebookVerifier(),
		auth.NewRedditVerifier(),
		auth.NewTwitchVerifier(),
		auth.NewTorusVerifier(),
	}
	if config.GlobalConfig.IsDebug {
		verifiers = append(verifiers, auth.NewTestVerifier("blublu"))
	}

	v.defaultVerifier = auth.NewGeneralVerifier(verifiers)
	return nil
}

func (v *VerifierService) Name() string {
	return "verifier"
}

func (v *VerifierService) OnStop() error {
	return nil
}

func (v *VerifierService) Call(method string, args ...interface{}) (interface{}, error) {
	telemetry.IncrementCounter(pcmn.TelemetryConstants.Generic.TotalServiceCalls, pcmn.TelemetryConstants.Verifier.Prefix)

	switch method {
	// Verify(rawMessage *bijson.RawMessage) (valid bool, verifierID string, timeIssued time.Time, err error)
	case "verify":
		telemetry.IncrementCounter(pcmn.TelemetryConstants.Verifier.VerifyCounter, pcmn.TelemetryConstants.Verifier.Prefix)

		// this has a custom marshaller
		var args0 bijson.RawMessage
		_ = castOrUnmarshal(args[0], &args0)
		rs := new(struct {
			Valid      bool
			VerifierID string
		})
		token := args0
		valid, verifierID, err := v.defaultVerifier.Verify(&token)
		rs.Valid = valid
		rs.VerifierID = verifierID
		return *rs, err
	// Lookup(verifierIdentifier string) (verifier auth.Verifier, err error)
	case "clean_token":
		telemetry.IncrementCounter(pcmn.TelemetryConstants.Verifier.CleanTokenCounter, pcmn.TelemetryConstants.Verifier.Prefix)

		var args0, args1 string
		_ = castOrUnmarshal(args[0], &args0)
		_ = castOrUnmarshal(args[1], &args1)

		verifier, err := v.defaultVerifier.Lookup(args0)
		if err != nil {
			return nil, err
		}
		return verifier.CleanToken(args1), nil
	// ListVerifiers() (verifiers []string)
	case "list_verifiers":
		telemetry.IncrementCounter(pcmn.TelemetryConstants.Verifier.ListVerifierCounter, pcmn.TelemetryConstants.Verifier.Prefix)

		verifiers := v.defaultVerifier.ListVerifiers()
		return verifiers, nil
	}
	return nil, fmt.Errorf("verifier service method %v not found", method)
}

func (v *VerifierService) SetBaseService(bs *BaseService) {
	v.bs = bs
}
