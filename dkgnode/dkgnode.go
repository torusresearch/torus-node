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
	Flags      *Flags
}

type Flags struct {
	Production bool
}

/* The entry point for our System */
func New(configPath string, register bool, production bool) {

	//Main suite of functions used in node
	suite := Suite{}
	suite.Flags = &Flags{production}
	fmt.Println(configPath)
	loadConfig(&suite, configPath)
	err := SetUpEth(&suite)
	if err != nil {
		log.Fatal(err)
	}
	setUpCache(&suite)
	var nodeIPAddress string

	if production {
		fmt.Println("//PRODUCTION MDOE ")
		nodeIPAddress = suite.Config.HostName + ":" + string(suite.Config.MyPort)
	} else {
		fmt.Println("//DEVELOPMENT MDOE ")
		nodeIP, err := findExternalIP()
		if err != nil {
			fmt.Println(err)
		}
		nodeIPAddress = nodeIP + ":" + string(suite.Config.MyPort)
	}

	fmt.Println("Node IP Address: " + nodeIPAddress)
	if register {
		/* Register Node */
		fmt.Println("Registering node...")
		_, err = suite.EthSuite.registerNode(nodeIPAddress)
		if err != nil {
			log.Fatal(err)
		}
	}

	go keyGenerationPhase(&suite)
	setUpServer(&suite, string(suite.Config.MyPort))
}
