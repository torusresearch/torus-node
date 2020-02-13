package main

import (
	logging "github.com/sirupsen/logrus"
	"github.com/torusresearch/torus-node/dkgnode"
	"github.com/torusresearch/torus-node/version"
)

func main() {
	/* Parse the provided parameters on command line */
	logging.WithField("version", version.NodeVersion).Info("TORUS NODE STARTING...")
	dkgnode.New()
}
