package common

type telemetryConstants struct {
	Generic    genericConstants
	ABCIApp    abciAppConstants
	ABCIServer abciServerConstants
	BFTRuleSet bftRuleSetConstants
	DB         dbConstants
	Cache      cacheConstants
	Ethereum   ethereumConstants
	JRPC       jrpcConstants
	Keygen     keygenConstants
	Mapping    mappingConstants
	Middleware middlewareConstants
	P2P        p2pConstants
	Ping       pingConstants
	PSS        pssConstants
	Server     serverConstants
	Tendermint tendermintConstants
	Verifier   verifierConstants
	Message    messageConstants
	Broadcast  broadcastConstants
}

type genericConstants struct {
	TotalServiceCalls string
}

type abciAppConstants struct {
	Prefix                          string
	CheckTxPrefix                   string
	TotalBftTxCounter               string
	RejectedBftTxCounter            string
	QueryCounter                    string
	GetIndexesFromVerifierIDCounter string
}

type abciServerConstants struct {
	Prefix                          string
	LastCreatedIndexCounter         string
	LastUnAssignedIndexCounter      string
	RetrieveKeyMappingCounter       string
	GetIndexesFromVerifierIdCounter string
	GetVerifierIteratorCounter      string
	GetVerifierIteratorNextCounter  string
}

type bftRuleSetConstants struct {
	Prefix                      string
	TransactionsCounter         string
	AssignmentCounter           string
	KeygenMessageCounter        string
	KeygenProposeCounter        string
	KeygenPubKeyCounter         string
	PSSMessageCounter           string
	PSSProposeCounter           string
	MappingMessageCounter       string
	MappingProposeFreezeCounter string
	MappingSummaryCounter       string
	MappingKeyCounter           string
	DealerMessageCounter        string
}

type dbConstants struct {
	Prefix                             string
	StoreKeygenCommitmentMatrixCounter string
	StorePSSCommitmentMatrixCounter    string
	StoreCompletedKeygenShareCounter   string
	StoreCompletedPssShareCounter      string
	StorePublicKeyToIndexCounter       string
	RetrieveCommitmentMatrixCounter    string
	RetrievePublicKeyToIndexCounter    string
	RetrieveIndexToPublicKeyCounter    string
	IndexToPublicKeyCounterExists      string
	RetrieveCompletedShareCounter      string
	GetShareCountCounter               string
	GetKeygenStartedCounter            string
	SetKeygenStartedCounter            string
	StoreNodePubKeyCounter             string
	RetrieveNodePubKeyCounter          string
	StoreConnectionDetailsCounter      string
	RetrieveConnectionDetailsCounter   string
}

type cacheConstants struct {
	Prefix                   string
	TokenCommitExistsCounter string
	GetTokenCommitKeyCounter string
	RecordTokenCommitCounter string
	SignerSigExistsCounter   string
	RecordSignerSigCounter   string
}

type ethereumConstants struct {
	Prefix                               string
	GetCurrentEpochCounter               string
	GetPreviousEpochCounter              string
	GetNextEpochCounter                  string
	GetEpochInfoCounter                  string
	GetSelfIndexCounter                  string
	GetSelfPrivateKeyCounter             string
	GetSelfPublicKeyCounter              string
	GetSelfAddressCounter                string
	SetSelfIndexCounter                  string
	SelfSignDataCounter                  string
	GetNodeListCounter                   string
	GetNodeDetailsByAddressCounter       string
	GetNodeDetailsByEpochAndIndexCounter string
	AwaitNodesConnectedCounter           string
	GetPSSStatusCounter                  string
	VerifyDataWithNodeListCounter        string
	VerifyDataWithEpochCounter           string
	StartPSSMonitorCounter               string
	GetTMP2PConnection                   string
	GetP2PConnection                     string
}

type jrpcConstants struct {
	Prefix                   string
	CommitmentRequestCounter string
	PingRequestCounter       string
	ConnectionDetailsCounter string
	ShareRequestCounter      string
	KeyAssignCounter         string
	VerifierLookupCounter    string
	KeyLookupCounter         string
	UpdatePublicKeyCounter   string
	UpdateShareCounter       string
	UpdateCommitmentCounter  string
}

type keygenConstants struct {
	Prefix                     string
	KeygenTotalCallCounter     string
	ReceiveMessageCounter      string
	ReceiveBFTMessageCounter   string
	ReceivedStreamMessage      string
	ProcessMessages            string
	ShareMessage               string
	SendMessage                string
	EchoMessage                string
	ReadyMessage               string
	CompleteMessage            string
	NizkpMessage               string
	DecideMessage              string
	IgnoredMessage             string
	AlreadyReceivedReady       string
	InvalidEcho                string
	InvalidReady               string
	ReadySigInvalid            string
	ReadyBeforeEcho            string
	SendingEcho                string
	SendingReady               string
	SendingComplete            string
	InvalidShareCommitment     string
	InvalidNizkp               string
	InvalidMethod              string
	SendingProposal            string
	ProcessedBroadcastMessages string
	IdUninitialized            string
	IdIncomplete               string
	NumSharesVerified          string
	DecidedSharingsNotComplete string
}

type mappingConstants struct {
	Prefix                          string
	NewMappingNodeCounter           string
	ReceiveBFTMessageCounter        string
	MappingInstanceExistsCounter    string
	GetMappingIDCounter             string
	GetFreezeStateCounter           string
	SetFreezeStateCounter           string
	ProposeFreezeCounter            string
	MappingSummaryFrozenCounter     string
	GetMappingProtocolPrefixCounter string
	Thawed                          string
	ReceiveSummaryMessage           string
	ReceiveKeyMessage               string
	ReceiveFrozenMessage            string
	SendSummary                     string
	SendKey                         string
	SummarySendBroadcast            string
	KeySendBroadcast                string
}

type middlewareConstants struct {
	Prefix                   string
	PingCounter              string
	CommitmentRequestCounter string
	ShareRequestCounter      string
	KeyAssignCounter         string
}

type p2pConstants struct {
	Prefix                            string
	GetPeerIDCounter                  string
	SetStreamHandlerCounter           string
	RemoveStreamHandlerCounter        string
	AuthenticateMessageCounter        string
	AuthenticateMessageInEpochCounter string
	NewP2PMessageCounter              string
	ConnectToP2PNodeCounter           string
	GetHostAddressCounter             string
	SignP2PMessageCounter             string
	SendP2PMessageCounter             string
}

type pingConstants struct {
	Prefix string
}

type pssConstants struct {
	Prefix                            string
	PSSInstanceExistsCounter          string
	GetPSSProtocolPrefixCounter       string
	GetNewNodesNCounter               string
	GetNewNodesKCounter               string
	GetNewNodesTCounter               string
	GetOldNodesNCounter               string
	GetOldNodesKCounter               string
	GetOldNodesTCounter               string
	NewPSSNodeCounter                 string
	SendPSSMessageToNodeCounter       string
	ReceiveBFTMessageCounter          string
	ProcessedMessageCounter           string
	IgnoredMessageCounter             string
	NotDealerCounter                  string
	NotPlayerCounter                  string
	SendingEchoCounter                string
	InvalidEchoCounter                string
	SendingReadyCounter               string
	AlreadyReceivedReadyCounter       string
	InvalidReadyCounter               string
	ReadySigInvalidCounter            string
	ReadyBeforeEchoCounter            string
	SendingCompleteCounter            string
	InsufficientRecoversCounter       string
	InvalidShareCommitmentCounter     string
	SendingProposalCounter            string
	InvalidMethodCounter              string
	ProcessedBroadcastMessagesCounter string
	DecidedSharingsNotCompleteCounter string
	FinalSharesInvalidCounter         string
	NumRefreshedCounter               string
}

type serverConstants struct {
	Prefix                          string
	RequestConnectionDetailsCounter string
}

type tendermintConstants struct {
	Prefix                 string
	GetNodeKeyCounter      string
	GetStatusCounter       string
	BroadcastCounter       string
	RegisterQueryCounter   string
	DeRegisterQueryCounter string
}

type verifierConstants struct {
	Prefix              string
	VerifyCounter       string
	CleanTokenCounter   string
	ListVerifierCounter string
}

type messageConstants struct {
	SentMessageCounter     string
	ReceivedMessageCounter string
}

type broadcastConstants struct {
	SentBroadcastCounter      string
	ReceivedBroadcastCounter  string
	BFTMessageQueueRunCounter string
}

var TelemetryConstants = telemetryConstants{
	Generic: genericConstants{
		TotalServiceCalls: "service_total",
	},
	ABCIApp: abciAppConstants{
		Prefix:                          "abci_app_",
		CheckTxPrefix:                   "abci_app_checktx_",
		TotalBftTxCounter:               "total_bft_tx_count",
		RejectedBftTxCounter:            "rejected_bft_tx_count",
		QueryCounter:                    "query_count_total",
		GetIndexesFromVerifierIDCounter: "query_count_get_indexes_from_verifier_id_total",
	},
	ABCIServer: abciServerConstants{
		Prefix:                          "abci_server",
		LastCreatedIndexCounter:         "service_count_last_created_index_total",
		LastUnAssignedIndexCounter:      "service_count_last_unassigned_index_total",
		RetrieveKeyMappingCounter:       "service_count_retrieve_key_mapping_total",
		GetIndexesFromVerifierIdCounter: "service_count_get_indexes_from_verifier_id_total",
		GetVerifierIteratorCounter:      "service_get_verifier_iterator_total",
		GetVerifierIteratorNextCounter:  "service_count_get_verifier_iterator_total",
	},
	BFTRuleSet: bftRuleSetConstants{
		Prefix:                      "bft_",
		TransactionsCounter:         "tx_total",
		AssignmentCounter:           "tx_assignment_total",
		KeygenMessageCounter:        "tx_keygen_message_total",
		KeygenProposeCounter:        "tx_keygen_propose_total",
		KeygenPubKeyCounter:         "tx_keygen_pubkey_total",
		PSSMessageCounter:           "tx_pss_message_total",
		PSSProposeCounter:           "tx_pss_propose_total",
		MappingMessageCounter:       "tx_mapping_message_total",
		MappingProposeFreezeCounter: "tx_mapping_propose_freeze_total",
		MappingSummaryCounter:       "tx_mapping_summary_total",
		MappingKeyCounter:           "tx_mapping_key_total",
		DealerMessageCounter:        "tx_dealer_message_total",
	},
	DB: dbConstants{
		Prefix:                             "db_",
		StoreKeygenCommitmentMatrixCounter: "service_store_keygen_commitment_matrix_total",
		StorePSSCommitmentMatrixCounter:    "service_store_pss_commitment_matrix_total",
		StoreCompletedKeygenShareCounter:   "service_completed_keygen_share_total",
		StoreCompletedPssShareCounter:      "service_completed_PSS_share_total",
		StorePublicKeyToIndexCounter:       "service_public_key_to_index_total",
		RetrieveCommitmentMatrixCounter:    "service_retrieve_commitment_matrix_total",
		RetrievePublicKeyToIndexCounter:    "service_retrieve_public_key_to_index_total",
		RetrieveIndexToPublicKeyCounter:    "service_retrieve_index_to_public_key_total",
		IndexToPublicKeyCounterExists:      "service_index_to_public_key_exists_total",
		RetrieveCompletedShareCounter:      "service_retrieve_completed_share_total",
		GetShareCountCounter:               "service_get_share_count_total",
		GetKeygenStartedCounter:            "service_get_keygen_started_total",
		SetKeygenStartedCounter:            "service_set_keygen_started_total",
		StoreNodePubKeyCounter:             "service_store_node_pub_key_total",
		RetrieveNodePubKeyCounter:          "service_retrieve_node_pub_key_total",
		StoreConnectionDetailsCounter:      "service_store_connection_details_total",
		RetrieveConnectionDetailsCounter:   "service_retrieve_connection_details_total",
	},
	Ethereum: ethereumConstants{
		Prefix:                               "ethereum_",
		GetCurrentEpochCounter:               "service_get_current_epoch_total",
		GetPreviousEpochCounter:              "service_get_previous_epoch_total",
		GetNextEpochCounter:                  "service_get_next_epoch_total",
		GetEpochInfoCounter:                  "service_get_epoch_info_total",
		GetSelfIndexCounter:                  "service_get_self_index_total",
		GetSelfPrivateKeyCounter:             "service_get_self_private_key_total",
		GetSelfPublicKeyCounter:              "service_get_self_public_key_total",
		GetSelfAddressCounter:                "service_get_self_address_total",
		SetSelfIndexCounter:                  "service_set_self_index_total",
		SelfSignDataCounter:                  "service_self_sign_data_total",
		GetNodeListCounter:                   "service_get_node_list_total",
		GetNodeDetailsByAddressCounter:       "service_get_node_details_by_address_total",
		GetNodeDetailsByEpochAndIndexCounter: "service_get_node_details_by_epoch_and_index_total",
		AwaitNodesConnectedCounter:           "service_await_nodes_connected_total",
		GetPSSStatusCounter:                  "service_get_PSS_status_total",
		VerifyDataWithNodeListCounter:        "service_verify_data_with_nodelist_total",
		VerifyDataWithEpochCounter:           "service_verify_data_with_epoch_total",
		StartPSSMonitorCounter:               "service_start_PSS_monitor_total",
		GetTMP2PConnection:                   "service_get_tm_p2p_connection",
		GetP2PConnection:                     "service_get_p2p_connection",
	},
	JRPC: jrpcConstants{
		Prefix:                   "jrpc_",
		CommitmentRequestCounter: "commitment_request_total",
		PingRequestCounter:       "ping_total",
		ConnectionDetailsCounter: "connection_details",
		ShareRequestCounter:      "share_request_total",
		KeyAssignCounter:         "key_assign_total",
		VerifierLookupCounter:    "verifier_lookup",
		KeyLookupCounter:         "key_lookup",
		UpdatePublicKeyCounter:   "update_public_key_total",
		UpdateShareCounter:       "update_share_total",
		UpdateCommitmentCounter:  "update_commitment_total",
	},
	Keygen: keygenConstants{
		Prefix:                     "keygen_",
		KeygenTotalCallCounter:     "service_call_total",
		ReceiveMessageCounter:      "receive_message_total",
		ReceiveBFTMessageCounter:   "receive_BFT_message_total",
		ReceivedStreamMessage:      "receive_stream_message_total",
		ProcessMessages:            "process_messages",
		ShareMessage:               "share_message",
		SendMessage:                "send_message",
		EchoMessage:                "echo_message",
		ReadyMessage:               "ready_message",
		CompleteMessage:            "complete_message",
		NizkpMessage:               "nizkp_message",
		DecideMessage:              "decide_message",
		IgnoredMessage:             "ignored_message",
		AlreadyReceivedReady:       "already_received_ready",
		InvalidEcho:                "invalid_echo",
		InvalidReady:               "invalid_ready",
		ReadySigInvalid:            "ready_sig_invalid",
		ReadyBeforeEcho:            "ready_before_echo",
		SendingEcho:                "sending_echo",
		SendingReady:               "sending_ready",
		SendingComplete:            "sending_complete",
		InvalidShareCommitment:     "invalid_share_commitment",
		InvalidNizkp:               "invalid_nizkp",
		InvalidMethod:              "invalid_method",
		SendingProposal:            "sending_proposal",
		ProcessedBroadcastMessages: "processed_broadcast_messages",
		IdUninitialized:            "id_uninitialized",
		IdIncomplete:               "id_incomplete",
		NumSharesVerified:          "num_shares_verified",
		DecidedSharingsNotComplete: "decided_sharings_not_complete",
	},
	Mapping: mappingConstants{
		Prefix:                          "mapping_",
		NewMappingNodeCounter:           "service_new_mapping_node_total",
		ReceiveBFTMessageCounter:        "service_receive_bft_message_total",
		MappingInstanceExistsCounter:    "service_mapping_instance_exists_total",
		GetMappingIDCounter:             "service_get_mapping_id_total",
		GetFreezeStateCounter:           "service_get_freeze_state_total",
		SetFreezeStateCounter:           "service_set_freeze_state_total",
		ProposeFreezeCounter:            "service_propose_freeze_total",
		MappingSummaryFrozenCounter:     "service_mapping_summary_frozen_total",
		GetMappingProtocolPrefixCounter: "service_get_mapping_protocol_prefix_total",
		Thawed:                          "thawed",
		ReceiveSummaryMessage:           "receive_summary_message",
		ReceiveKeyMessage:               "receive_key_message",
		ReceiveFrozenMessage:            "receive_frozen_message",
		SendSummary:                     "send_summary",
		SendKey:                         "send_key",
		SummarySendBroadcast:            "summary_send_broadcast",
		KeySendBroadcast:                "key_send_broadcast",
	},
	Middleware: middlewareConstants{
		Prefix:                   "middleware_",
		PingCounter:              "ping_count",
		CommitmentRequestCounter: "commitment_request_count",
		ShareRequestCounter:      "share_request_count",
		KeyAssignCounter:         "key_assign_count",
	},
	P2P: p2pConstants{
		Prefix:                            "p2p_",
		GetPeerIDCounter:                  "service_get_peer_id_total",
		SetStreamHandlerCounter:           "service_set_stream_handler_total",
		RemoveStreamHandlerCounter:        "service_remove_stream_handler_total",
		AuthenticateMessageCounter:        "service_authenticate_message_total",
		AuthenticateMessageInEpochCounter: "service_authenticate_message_in_epoch_total",
		NewP2PMessageCounter:              "service_new_p2p_message_total",
		ConnectToP2PNodeCounter:           "service_connect_to_p2p_node_total",
		GetHostAddressCounter:             "service_get_host_address_total",
		SignP2PMessageCounter:             "service_sign_p2p_message_total",
		SendP2PMessageCounter:             "service_send_p2p_message_total",
	},
	Ping: pingConstants{
		Prefix: "ping_",
	},
	PSS: pssConstants{
		Prefix:                            "pss_",
		PSSInstanceExistsCounter:          "service_pss_instance_exists_total",
		GetPSSProtocolPrefixCounter:       "service_get_PSS_protocol_prefix_total",
		GetNewNodesNCounter:               "service_get_new_nodes_n_total",
		GetNewNodesKCounter:               "service_get_new_nodes_k_total",
		GetNewNodesTCounter:               "service_get_new_nodes_t_total",
		GetOldNodesNCounter:               "service_get_old_nodes_n_total",
		GetOldNodesKCounter:               "service_get_old_nodes_k_total",
		GetOldNodesTCounter:               "service_get_old_nodes_t_total",
		NewPSSNodeCounter:                 "service_new_PSS_node_total",
		SendPSSMessageToNodeCounter:       "service_send_pss_message_to_node_total",
		ReceiveBFTMessageCounter:          "receive_BFT_message",
		ProcessedMessageCounter:           "processed_messages",
		IgnoredMessageCounter:             "ignored_message",
		NotDealerCounter:                  "not_dealer",
		NotPlayerCounter:                  "not_player",
		SendingEchoCounter:                "sending_echo",
		InvalidEchoCounter:                "invalid_echo",
		SendingReadyCounter:               "sending_ready",
		AlreadyReceivedReadyCounter:       "already_received_ready",
		InvalidReadyCounter:               "invalid_ready",
		ReadySigInvalidCounter:            "ready_sig_invalid",
		ReadyBeforeEchoCounter:            "ready_before_echo",
		SendingCompleteCounter:            "sending_complete",
		InsufficientRecoversCounter:       "insufficient_recovers",
		InvalidShareCommitmentCounter:     "invalid_share_commitment",
		SendingProposalCounter:            "sending_proposal",
		InvalidMethodCounter:              "invalid_method",
		ProcessedBroadcastMessagesCounter: "processed_broadcast_messages",
		DecidedSharingsNotCompleteCounter: "decided_sharings_not_complete",
		FinalSharesInvalidCounter:         "final_shares_invalid",
		NumRefreshedCounter:               "num_refreshed",
	},
	Server: serverConstants{
		Prefix:                          "server_",
		RequestConnectionDetailsCounter: "request_connection_details_total",
	},
	Tendermint: tendermintConstants{
		Prefix:                 "tendermint_",
		GetNodeKeyCounter:      "service_get_node_key_total",
		GetStatusCounter:       "service_get_status_total",
		BroadcastCounter:       "service_get_status_total",
		RegisterQueryCounter:   "service_register_query_total",
		DeRegisterQueryCounter: "service_deregister_query_total",
	},
	Verifier: verifierConstants{
		Prefix:              "verifier_",
		VerifyCounter:       "service_verify_total",
		CleanTokenCounter:   "service_clean_token_total",
		ListVerifierCounter: "service_list_verifiers_total",
	},
	Cache: cacheConstants{
		Prefix:                   "cache_",
		TokenCommitExistsCounter: "service_token_commit_exists_total",
		GetTokenCommitKeyCounter: "service_get_token_commit_key_total",
		RecordTokenCommitCounter: "service_record_token_commit_total",
		SignerSigExistsCounter:   "signer_sig_exists_total",
		RecordSignerSigCounter:   "record_signer_sig_total",
	},
	Message: messageConstants{
		SentMessageCounter:     "message_sent_total",
		ReceivedMessageCounter: "message_received_total",
	},
	Broadcast: broadcastConstants{
		SentBroadcastCounter:      "broadcast_sent_total",
		ReceivedBroadcastCounter:  "broadcast_received_total",
		BFTMessageQueueRunCounter: "broadcast_bft_msg_queue_total",
	},
}
