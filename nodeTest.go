package main

/* Al useful imports */
import (
	"flag"
	"fmt"
	"log"
)

type Suite struct {
	EthSuite   *EthSuite
	CacheSuite *CacheSuite
	Config     *Config
}

/* The entry point for our System */
func main() {
	/* Parse the provided parameters on command line */
	configPath := flag.String("configPath", "./node/config.json", "provide path to config file, defaults ./node/config.json")
	flag.Parse()

	//Main suite of functions used in node
	suite := Suite{}

	loadConfig(&suite, *configPath)
	err := setUpEth(&suite)
	if err != nil {
		log.Fatal(err)
	}

	/* Register Node */
	nodeIP, err := findExternalIP()
	if err != nil {
		fmt.Println(err)
	}
	fmt.Println("Node IP Address: " + nodeIP + ":" + string(suite.Config.MyPort))
	_, err = suite.EthSuite.registerNode(nodeIP + ":" + string(suite.Config.MyPort))
	if err != nil {
		log.Fatal(err)
	}

	test := make([]string, 1)
	test[0] = "http://localhost:" + string(suite.Config.MyPort) + "/jrpc"
	// go setUpClient(test)
	go keyGenerationPhase(&suite)
	setUpServer(*suite.EthSuite, string(suite.Config.MyPort))

}
