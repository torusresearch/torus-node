package dkgnode

/* All useful imports */
import (
	"fmt"
	"strings"

	"github.com/micro/go-config"
	"github.com/micro/go-config/source/file"
)

type Config struct {
	MyPort                     string `json:"myport"`
	MainServerAddress          string `json:"mainserveraddress"`
	EthConnection              string `json:"ethconnection"`
	EthPrivateKey              string `json:"ethprivatekey"`
	BftURI                     string `json:"bfturi`
	ABCIServer                 string `json:"abciserver`
	P2PListenAddress           string `json"p2plistenaddress"`
	NodeListAddress            string `json:"nodelistaddress"`
	HostName                   string `json:"hostname"`
	NumberOfNodes              int    `json:"numberofnodes"`
	Threshold                  int    `json:"threshold"`
	KeysPerEpoch               int    `json:"keysperepoch"`
	KeyBufferTriggerPercentage int    `json:"keybuffertriggerpercetage"` //percetage threshold of keys left to trigger buffering 90 - 20
	BuildPath                  string `json:"buildpath"`
}

func loadConfig(suite *Suite, path string, nodeAddress string, privateKey string, buildPath string, nodeListAddress string) {

	conf := defaultConfigSettings()
	nodeIP, err := findExternalIP()
	if err != nil {
		fmt.Println(err)
	}

	if path != "" {
		fmt.Println("Running on Configuration File")
		/* Load Config */
		config.Load(file.NewSource(
			file.WithPath(path),
		))
		config.Scan(&conf)
		conf.MainServerAddress = nodeIP + ":" + conf.MyPort
		// retrieve map[string]interface{}
	} else if nodeAddress != "" {
		fmt.Println("Running on Specified IP Address")
		//Specified for docker configurations
		// conf.BftURI = "tcp://" + nodeAddress + ":" + strings.Split(conf.BftURI, ":")[2]
		// conf.ABCIServer = "tcp://" + nodeAddress + ":" + strings.Split(conf.ABCIServer, ":")[2]
		// conf.P2PListenAddress = "tcp://" + nodeAddress + ":" + strings.Split(conf.P2PListenAddress, ":")[2]
		conf.MainServerAddress = nodeAddress + ":" + conf.MyPort
	} else {
		fmt.Println("Running on Default Configurations")
		//In default configurations we find server IP
		conf.BftURI = "tcp://" + nodeIP + ":" + strings.Split(conf.BftURI, ":")[2]
		conf.ABCIServer = "tcp://" + nodeIP + ":" + strings.Split(conf.ABCIServer, ":")[2]
		conf.P2PListenAddress = "tcp://" + nodeIP + ":" + strings.Split(conf.P2PListenAddress, ":")[2]
		conf.MainServerAddress = nodeIP + ":" + conf.MyPort
	}

	//replace config private key if flat is provided
	//TODO: validation checks on private key
	if privateKey != "" {
		conf.EthPrivateKey = privateKey
	}
	if buildPath != "" && buildPath != "./.build" {
		conf.BuildPath = buildPath
	}
	if nodeListAddress != "" {
		conf.NodeListAddress = nodeListAddress
	}
	fmt.Println("Configuration: ")
	fmt.Printf("%+v\n", conf)
	//edit the config to use nodeAddress
	suite.Config = &conf
}

func defaultConfigSettings() Config {
	return Config{
		MyPort:                     "80",
		MainServerAddress:          "127.0.0.1:80",
		EthConnection:              "http://178.128.178.162:8545",
		EthPrivateKey:              "29909a750dc6abc3e3c83de9c6da9d6faf9fde4eebb61fa21221415557de5a0b",
		BftURI:                     "tcp://0.0.0.0:26657",
		ABCIServer:                 "tcp://0.0.0.0:8010",
		P2PListenAddress:           "tcp://0.0.0.0:26656",
		NodeListAddress:            "0x35e47457b49ff23c8cb241f422bc79d78efb6a13",
		HostName:                   "",
		NumberOfNodes:              5,
		Threshold:                  3,
		KeysPerEpoch:               100,
		KeyBufferTriggerPercentage: 80,
		BuildPath:                  "/.build",
	}
}
