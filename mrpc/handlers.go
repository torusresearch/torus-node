package mrpc

import (
	"github.com/torusresearch/jsonrpc"
	"github.com/torusresearch/torus-node/dealer"
)

type (
	SetLogLevelHandler struct{}
	SetLogLevelParams  struct {
		Level string `json:"level"`
	}
	SetLogLevelResult struct{}

	SetMutableConfigHandler struct{}
	SetMutableConfigParams  struct {
		Key         string `json:"key"`
		ValueType   string `json:"value_type"`
		StringValue string `json:"string_value"`
		IntValue    int    `json:"int_value"`
		BoolValue   bool   `json:"bool_value"`
	}
	SetMutableConfigResult struct {
		NewConfigs string `json:"new_configs"`
	}
	RetriggerPSSHandler struct {
		RetriggerPSS RetriggerPSSAction
	}
	RetriggerPSSParams struct {
		PssProtocolPrefix string `json:"pss_protocol_prefix"`
		EndIndex          int    `json:"end_index"`
		EpochOld          int    `json:"epoch_old"`
		NOld              int    `json:"n_old"`
		KOld              int    `json:"k_old"`
		TOld              int    `json:"t_old"`
		EpochNew          int    `json:"epoch_new"`
		NNew              int    `json:"n_new"`
		KNew              int    `json:"k_new"`
		TNew              int    `json:"t_new"`
	}
	RetriggerPSSResult struct{}
	Action             func(interface{}) (interface{}, error)
	Actions            struct {
		RetriggerPSS        RetriggerPSSAction
		HandleDealerMessage HandleDealerMessage
	}
	RetriggerPSSAction  func() error
	HandleDealerMessage func(dealer.Message) error

	DealerMessageHandler struct {
		HandleDealerMessage HandleDealerMessage
	}
	DealerMessageParams struct {
		DealerMessage dealer.Message `json:"dealerMessage"`
	}
	DealerMessageResult struct {
		Result string
	}
)

func SetupManagementRPCHander(actions Actions) (*jsonrpc.MethodRepository, error) {
	mr := jsonrpc.NewMethodRepository()

	err := mr.RegisterMethod(
		"SetLogLevel",
		SetLogLevelHandler{},
		SetLogLevelParams{},
		SetLogLevelResult{},
	)
	if err != nil {
		return nil, err
	}

	err = mr.RegisterMethod(
		"SetMutableConfig",
		SetMutableConfigHandler{},
		SetMutableConfigParams{},
		SetMutableConfigResult{},
	)
	if err != nil {
		return nil, err
	}

	err = mr.RegisterMethod(
		"RetriggerPSS",
		RetriggerPSSHandler{RetriggerPSS: actions.RetriggerPSS},
		RetriggerPSSParams{},
		RetriggerPSSResult{},
	)
	if err != nil {
		return nil, err
	}

	return mr, nil
}
