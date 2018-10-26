package main

/* Al useful imports */
import (
	"flag"
	"fmt"
	"log"
)

/* The entry point for our System */
func main() {
	/* Parse the provided parameters on command line */
	configPath := flag.String("configPath", "./node/config.json", "provide path to config file, defaults ./node/config.json")
	flag.Parse()

	conf := loadConfig(*configPath)
	ethSuite, err := setUpEth(conf)
	if err != nil {
		log.Fatal(err)
	}

	/* Register Node */
	nodeIp, err := findExternalIP()
	if err != nil {
		fmt.Println(err)
	}
	fmt.Println("Node IP Address: " + nodeIp + ":" + string(conf.MyPort))
	_, err = ethSuite.registerNode(nodeIp + ":" + string(conf.MyPort))
	if err != nil {
		log.Fatal(err)
	}

	test := make([]string, 1)
	test[0] = "http://localhost:" + string(conf.MyPort) + "/jrpc"
	// go setUpClient(test)
	go keyGenerationPhase(ethSuite)
	setUpServer(*ethSuite, string(conf.MyPort))

}
