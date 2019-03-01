package tmabci

import (
	"os"

	"github.com/YZhenY/tendermint/abci/server"
	"github.com/YZhenY/tendermint/abci/types"
	"github.com/YZhenY/tendermint/libs/common"
	"github.com/YZhenY/tendermint/libs/log"
)

func RunABCIServer() error {
	logger := log.NewTMLogger(log.NewSyncWriter(os.Stdout))

	// Create the application - in memory or persisted to disk
	var app types.Application
	// if flagPersist == "" {
	app = NewKVStoreApplication()
	// } else {
	// 	app = kvstore.NewPersistentKVStoreApplication(flagPersist)
	// 	app.(*kvstore.PersistentKVStoreApplication).SetLogger(logger.With("module", "kvstore"))
	// }

	// Start the listener
	//TODO: change literals to flags
	srv, err := server.NewServer("tcp://0.0.0.0:26658", "socket", app)
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
