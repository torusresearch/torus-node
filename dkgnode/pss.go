package dkgnode

type PSSWorkerUpdate struct {
	Type    string
	Payload interface{}
}

func pssWorker(suite *Suite, pssMonitorMsgs <-chan PSSWorkerUpdate) {
	for pssMonitorMsgs := range pssMonitorMsgs {
		if pssMonitorMsgs.Type == "all_connected" {

		}
	}
	return
}

type PSSSuite struct {
	// PSSNode *pss.PSSNode
}

func SetupPSS(suite *Suite) {
	suite.PSSSuite = &PSSSuite{
		// PSSNode: pss.NewPSSNode(
		// 	common.Node{
		// 		Index:  *suite.EthSuite.NodeIndex, // TODO: check if this is loaded
		// 		PubKey: *suite.EthSuite.NodePublicKey,
		// 	},
		// ),
	}
}
