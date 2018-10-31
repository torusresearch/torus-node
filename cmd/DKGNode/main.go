package main

import (
	"flag"

	"github.com/YZhenY/DKGNode/dkgnode"
)

func main() {
	/* Parse the provided parameters on command line */
	register := flag.Bool("register", true, "defaults true")
	configPath := flag.String("configPath", "./config.json", "provide path to config file, defaults ./cmd/DKGNode/config.json")
	flag.Parse()
	dkgnode.New(*configPath, *register)
}
