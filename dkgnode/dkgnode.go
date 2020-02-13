package dkgnode

import (
	"context"
	"net/http"
	"time"

	// _ "net/http/pprof"
	"os"
	"os/signal"
	"runtime/debug"
	"syscall"

	logging "github.com/sirupsen/logrus"
	"github.com/stackimpact/stackimpact-go"
	"github.com/torusresearch/torus-node/config"
	"github.com/torusresearch/torus-node/eventbus"
	"github.com/torusresearch/torus-node/tcontext"
	"github.com/torusresearch/torus-node/telemetry"
	"github.com/torusresearch/torus-node/version"
)

const defaultConfigPath = "/.torus/config.json"

// New initializes the Torus node, spawns and sets up appropriate services

func New() {
	systemEventBus := eventbus.New()
	ctx, cancel := context.WithCancel(context.Background())
	systemContext := context.WithValue(ctx, tcontext.ContextID, 1)
	logging.RegisterExitHandler(func() {
		debug.PrintStack()
	})
	logging.WithFields(logging.Fields{
		"systemEventBus": systemEventBus,
		"systemContext":  systemContext,
	}).Info("initialized system event bus and system context")

	// Load configs
	config.GlobalConfig = config.LoadConfig(defaultConfigPath)
	config.GlobalMutableConfig = config.InitMutableConfig(config.GlobalConfig)
	logging.WithFields(logging.Fields{
		"BftURI":             config.GlobalConfig.BftURI,
		"MainServerAddress":  config.GlobalConfig.MainServerAddress,
		"tmp2plistenaddress": config.GlobalConfig.TMP2PListenAddress,
	}).Info("loaded config")

	if config.GlobalConfig.StackImpactEnabled {
		// config.GlobalConfig.PprofEnabled = false
		stackimpact.Start(stackimpact.Options{
			AgentKey:         config.GlobalConfig.StackImpactKey,
			AppName:          "dkgnode" + version.NodeVersion + config.GlobalConfig.PublicURL,
			DashboardAddress: config.GlobalConfig.PublicURL,
			HostName:         config.GlobalConfig.PublicURL,
		})
	}

	// Profiling for node, will never be enabled from production nodes
	if config.GlobalConfig.IsDebug && config.GlobalConfig.PprofEnabled {
		logging.Info("RUNNING IN DEBUG MODE")
		go func() {
			logging.Info("Starting pprof...")
			err := http.ListenAndServe("0.0.0.0:"+config.GlobalConfig.PprofPort, nil)
			logging.WithError(err).Warn()
		}()
	}

	// Start services
	telemetryService := NewTelemetryService(systemContext, systemEventBus)
	ethereumService := NewEthereumService(systemContext, systemEventBus)
	abciService := NewABCIService(systemContext, systemEventBus)
	tendermintService := NewTendermintService(systemContext, systemEventBus)
	p2pService := NewP2PService(systemContext, systemEventBus)
	serverService := NewServerService(systemContext, systemEventBus)
	keygennofsmService := NewKeygennofsmService(systemContext, systemEventBus)
	pssService := NewPSSService(systemContext, systemEventBus)
	mappingService := NewMappingService(systemContext, systemEventBus)
	verifierService := NewVerifierService(systemContext, systemEventBus)
	databaseService := NewDatabaseService(systemContext, systemEventBus)
	cacheService := NewCacheService(systemContext, systemEventBus)

	serviceRegistry := NewServiceRegistry(
		systemEventBus,
		telemetryService,
		ethereumService,
		abciService,
		tendermintService,
		p2pService,
		serverService,
		keygennofsmService,
		pssService,
		mappingService,
		verifierService,
		databaseService,
		cacheService,
	)
	setupRequestLoggingMiddleware(serviceRegistry)
	// setupDiskQueueMiddleware(serviceRegistry)
	setupResponseLoggingMiddleware(serviceRegistry)

	for baseServiceName, baseService := range serviceRegistry.Services {
		go func(name string, service *BaseService) {
			logging.WithField("name", name).Debug("baseService start")
			ok, err := service.Start()
			if err != nil {
				logging.WithField("name", name).WithError(err).Fatal("could not start baseService")
			}
			if !ok {
				logging.WithField("name", name).Fatal("baseService has already been started")
			}
			logging.WithField("name", name).Debug("baseService started")
		}(baseServiceName, baseService)
	}

	go func() {
		for {
			time.Sleep(time.Duration(config.GlobalMutableConfig.GetI("OSFreeMemoryMS")) * time.Millisecond)
			debug.FreeOSMemory()
		}
	}()

	go func() {
		telemetry.IncrementCounter(version.GitCommit, "torus_git_commit")
	}()

	idleConnsClosed := make(chan struct{})

	// Stop upon receiving SIGTERM or CTRL-C
	osSignal := make(chan os.Signal, 1)
	signal.Notify(osSignal, os.Interrupt, syscall.SIGTERM, syscall.SIGINT)
	go func() {
		osSig := <-osSignal

		logging.Info("shutting down the node, received signal: " + osSig.String())
		cancel()

		// Exit the blocking chan
		close(idleConnsClosed)
	}()

	<-idleConnsClosed
}
