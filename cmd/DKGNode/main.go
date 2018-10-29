package main

import (
	"flag"

	"github.com/YZhenY/DKGNode/dkgnode"
)

func main() {
	/* Parse the provided parameters on command line */
	configPath := flag.String("configPath", "./cmd/DKGNode/config.json", "provide path to config file, defaults ./cmd/DKGNode/config.json")
	flag.Parse()
	dkgnode.New(*configPath)
}
