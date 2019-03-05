package main

import (
	"github.com/torusresearch/torus-public/dkgnode"
	"github.com/torusresearch/torus-public/logging"
)

func main() {
	/* Parse the provided parameters on command line */
	// TODO: NodeVersion should be handled a bit differently
	logging.Infof("---- STARTING TORUS NODE v%s ----", NodeVersion)
	dkgnode.New()
}
