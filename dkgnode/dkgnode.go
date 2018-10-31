package dkgnode

/* Al useful imports */
import (
	"fmt"
	"log"
)

type Suite struct {
	EthSuite   *EthSuite
	CacheSuite *CacheSuite
	Config     *Config
}

/* The entry point for our System */
func New(configPath string, register bool) {

	//Main suite of functions used in node
	suite := Suite{}

	loadConfig(&suite, configPath)
	err := SetUpEth(&suite)
	if err != nil {
		log.Fatal(err)
	}
	setUpCache(&suite)

	/* Register Node */
	// nodeIP, err := findExternalIP()
	// if err != nil {
	// 	fmt.Println(err)
	// }
	fmt.Println("Node IP Address: " + suite.Config.HostName + ":" + string(suite.Config.MyPort))
	if register {
		fmt.Println("Registering node...")
		_, err = suite.EthSuite.registerNode(suite.Config.HostName + ":" + string(suite.Config.MyPort))
		if err != nil {
			log.Fatal(err)
		}
	}

	// test := make([]string, 1)
	// test[0] = "http://localhost:" + string(suite.Config.MyPort) + "/jrpc"
	// go setUpClient(test)
	go keyGenerationPhase(&suite)
	setUpServer(&suite, string(suite.Config.MyPort))
}
