package main

import (
	"flag"

	"github.com/YZhenY/DKGNode/dkgnode"
)

func main() {
	/* Parse the provided parameters on command line */
	register := flag.Bool("register", true, "defaults to true")
	production := flag.Bool("production", false, "defaults to false")
	configPath := flag.String("configPath", "", "provide path to config file, defaults ./cmd/DKGNode/config.json")

	flag.Parse()
	dkgnode.New(*configPath, *register, *production)
}
