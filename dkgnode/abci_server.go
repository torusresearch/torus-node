package dkgnode

import (
	"os"

	"github.com/tendermint/tendermint/abci/server"
	"github.com/tendermint/tendermint/libs/common"
	"github.com/tendermint/tendermint/libs/log"
)

func RunABCIServer(suite *Suite) error {
	logger := log.NewTMLogger(log.NewSyncWriter(os.Stdout))

	suite.ABCIApp = NewABCIApp(suite)
	// Start the listener
	srv, err := server.NewServer(suite.Config.ABCIServer, "socket", suite.ABCIApp)
	if err != nil {
		return err
	}

	srv.SetLogger(logger.With("module", "abci-server"))
	if err := srv.Start(); err != nil {
		return err
	}

	// Wait forever
	common.TrapSignal(func() {
		// Cleanup
		srv.Stop()
	})
	return nil
}
