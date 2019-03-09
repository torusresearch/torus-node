package dkgnode

/* All useful imports */
import (
	"encoding/json"
	"flag"
	"io/ioutil"
	"os"

	"github.com/caarlos0/env"
	"github.com/torusresearch/torus-public/logging"
)

type Config struct {
	MyPort                     string `json:"myport" env:"MYPORT"`
	MainServerAddress          string `json:"mainserveraddress" env:"MAIN_SERVER_ADDRESS"`
	EthConnection              string `json:"ethconnection" env:"ETH_CONNECTION"`
	EthPrivateKey              string `json:"ethprivatekey" env:"ETH_PRIVATE_KEY"`
	BftURI                     string `json:"bfturi" env:"BFT_URI"`
	ABCIServer                 string `json:"abciserver" env:"ABCI_SERVER"`
	P2PListenAddress           string `json:"p2plistenaddress" env:"P2P_LISTEN_ADDRESS"`
	NodeListAddress            string `json:"nodelistaddress" env:"NODE_LIST_ADDRESS"`
	NumberOfNodes              int    `json:"numberofnodes" env:"NUMBER_OF_NODES"`
	Threshold                  int    `json:"threshold" env:"THRESHOLD"`
	KeysPerEpoch               int    `json:"keysperepoch" env:"KEYS_PER_EPOCH"`
	KeyBufferTriggerPercentage int    `json:"keybuffertriggerpercetage" env:"KEY_BUFFER_TRIGGER_PERCENTAGE"` //percetage threshold of keys left to trigger buffering 90 - 20
	BasePath                   string `json:"basepath" env:"BASE_PATH"`

	ShouldRegister    bool   `json:"register" env:"REGISTER"`
	CPUProfileToFile  string `json:"cpuProfile" env:"CPU_PROFILE"`
	IsProduction      bool   `json:"production" env:"PRODUCTION"`
	ProvidedIPAddress string `json:"ipAddress" env:"IP_ADDRESS"`
	LogLevel          string `json:"loglevel" env:"LOG_LEVEL"`
}

// mergeWithFlags explicitly merges flags for a given instance of Config
// NOTE: It will note override with defaults
func (c *Config) mergeWithFlags() *Config {
	register := flag.Bool("register", true, "defaults to true")
	production := flag.Bool("production", false, "defaults to false")
	ethPrivateKey := flag.String("ethprivateKey", "", "provide private key here to run node on")
	ipAddress := flag.String("ipAddress", "", "specified IPAdress, necessary for running in an internal env e.g. docker")
	cpuProfile := flag.String("cpuProfile", "", "write cpu profile to file")
	ethConnection := flag.String("ethConnection", "", "ethereum endpoint")
	nodeListAddress := flag.String("nodeListAddress", "", "node list address on ethereum")
	basePath := flag.String("basePath", "/.torus", "basePath for Torus node artifacts")

	flag.Parse()

	if isFlagPassed("register") {
		c.ShouldRegister = *register
	}
	if isFlagPassed("production") {
		c.IsProduction = *production
	}
	if isFlagPassed("ethprivateKey") {
		c.EthPrivateKey = *ethPrivateKey
	}
	if isFlagPassed("ipAddress") {
		c.ProvidedIPAddress = *ipAddress
	}
	if isFlagPassed("cpuProfile") {
		c.CPUProfileToFile = *cpuProfile
	}
	if isFlagPassed("ethConnection") {
		c.EthConnection = *ethConnection
	}
	if isFlagPassed("nodeListAddress") {
		c.NodeListAddress = *nodeListAddress
	}
	if isFlagPassed("basePath") {
		c.BasePath = *basePath
	}

	return c
}

// Source: https://stackoverflow.com/a/54747682
func isFlagPassed(name string) bool {
	found := false
	flag.Visit(func(f *flag.Flag) {
		if f.Name == name {
			found = true
		}
	})
	return found
}

func readAndMarshallJSONConfig(configPath string, c *Config) error {
	jsonConfig, err := os.Open(configPath)
	if err != nil {
		return err
	}

	defer jsonConfig.Close()

	b, err := ioutil.ReadAll(jsonConfig)
	if err != nil {
		return err
	}

	err = json.Unmarshal(b, &c)
	if err != nil {
		return err
	}

	return nil
}

func loadConfig(configPath string) *Config {

	// Default config is initalized here
	conf := defaultConfigSettings()

	nodeIP, err := findExternalIP()
	if err != nil {
		// QUESTION(TEAM) - unhandled error, was only fmt.Printlnd
		logging.Errorf("%s", err)
	}

	providedCF := *flag.String("configPath", "", "override configPath")
	if providedCF != "" {
		logging.Infof("overriding configPath to: %s", providedCF)
		configPath = providedCF
	}

	err = readAndMarshallJSONConfig(configPath, &conf)
	if err != nil {
		logging.Warningf("failed to read JSON config with err: %s", err)
	}

	err = env.Parse(&conf)
	if err != nil {
		logging.Error(err.Error())
	}

	conf.mergeWithFlags()

	logging.SetLevelString(conf.LogLevel)

	//if in production use configured nodeIP for server. else use https dev host address "localhost"
	if conf.IsProduction {
		conf.MainServerAddress = nodeIP + ":" + conf.MyPort
	} else {
		conf.MainServerAddress = "localhost" + ":" + conf.MyPort
	}
	// retrieve map[string]interface{}
	if conf.ProvidedIPAddress != "" {
		logging.Infof("Running on Specified IP Address: %s", conf.ProvidedIPAddress)
		conf.MainServerAddress = conf.ProvidedIPAddress + ":" + conf.MyPort
	}

	return &conf
}

func defaultConfigSettings() Config {
	return Config{
		MyPort:                     "443",
		MainServerAddress:          "127.0.0.1:443",
		EthConnection:              "http://178.128.178.162:14103",
		EthPrivateKey:              "29909a750dc6abc3e3c83de9c6da9d6faf9fde4eebb61fa21221415557de5a0b",
		BftURI:                     "tcp://0.0.0.0:26657",
		ABCIServer:                 "tcp://0.0.0.0:8010",
		P2PListenAddress:           "tcp://0.0.0.0:26656",
		NodeListAddress:            "0x4e8fce1336c534e0452410c2cb8cd628949dcc85",
		NumberOfNodes:              5,
		Threshold:                  3,
		KeysPerEpoch:               100,
		KeyBufferTriggerPercentage: 80,
		BasePath:                   "/.torus",
		IsProduction:               false,
		LogLevel:                   "debug",
	}
}
