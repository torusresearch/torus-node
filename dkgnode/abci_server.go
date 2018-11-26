package dkgnode

import (
	"os"

	"github.com/tendermint/tendermint/abci/server"
	"github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/libs/common"
	"github.com/tendermint/tendermint/libs/log"
)

func RunABCIServer(suite *Suite) error {
	logger := log.NewTMLogger(log.NewSyncWriter(os.Stdout))

	// Create the application - in memory or persisted to disk
	var app types.Application
	// if flagPersist == "" {
	app = NewABCIApp(suite)
	// } else {
	// 	app = kvstore.NewPersistentKVStoreApplication(flagPersist)
	// 	app.(*kvstore.PersistentKVStoreApplication).SetLogger(logger.With("module", "kvstore"))
	// }

	// Start the listener
	srv, err := server.NewServer(suite.Config.ABCIServer, "socket", app)
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
