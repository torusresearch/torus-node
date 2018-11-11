package main

import (
	"errors"
	"flag"
	"log"

	"github.com/YZhenY/DKGNode/dkgnode"
)

func main() {
	/* Parse the provided parameters on command line */
	register := flag.Bool("register", true, "defaults to true")
	production := flag.Bool("production", false, "defaults to false")
	configPath := flag.String("configPath", "", "provide path to config file")

	flag.Parse()
	if *configPath == "" {
		log.Fatal(errors.New("No configuration path provided, aborting"))
	} else {
		dkgnode.New(*configPath, *register, *production)
	}
}
