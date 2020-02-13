package main

import (
	logging "github.com/sirupsen/logrus"
	"github.com/torusresearch/torus-public/dkgnode"
	"github.com/torusresearch/torus-public/version"
)

func main() {
	/* Parse the provided parameters on command line */
	logging.WithField("version", version.NodeVersion).WithField().Info("TORUS NODE STARTING...")
	dkgnode.New()
}
