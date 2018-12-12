package main

import (
	"flag"
	"fmt"

	"github.com/torusresearch/torus/dkgnode"
)

func main() {
	/* Parse the provided parameters on command line */
	register := flag.Bool("register", true, "defaults to true")
	production := flag.Bool("production", false, "defaults to false")
	configPath := flag.String("configPath", "", "provide path to config file")
	buildPath := flag.String("buildPath", "./.build", "provide path to build file")
	privateKey := flag.String("privateKey", "", "provide private key here to run node on")
	nodeIPAddress := flag.String("ipAddress", "", "specified IPAdress, necessary for running in an internal env e.g. docker")
	cpuProfile := flag.String("cpuProfile", "", "write cpu profile to file")
	nodeListAddress := flag.String("nodeListAddress", "", "node list address on ethereum")
	flag.Parse()
	fmt.Println("---- STARTING TORUS NODE v0.0.8 ----")

	dkgnode.New(*configPath, *register, *production, *buildPath, *cpuProfile, *nodeIPAddress, *privateKey, *nodeListAddress)
}
