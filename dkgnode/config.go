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
	HttpServerPort string `json:"httpServerPort" env:"HTTP_SERVER_PORT"`
	// NOTE: This is what is used for registering on the Ethereum network.
	MainServerAddress          string `json:"mainServerAddress" env:"MAIN_SERVER_ADDRESS"`
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

	ServeUsingTLS bool   `json:"USE_TLS" env:"USE_TLS"`
	UseAutoCert   bool   `json:"useAutoCert" env:"USE_AUTO_CERT"`
	ServerCert    string `json:"serverCert" env:"SERVER_CERT"`
	ServerKey     string `json:"serverKey" env:"SERVER_KEY"`
}

// mergeWithFlags explicitly merges flags for a given instance of Config
// NOTE: It will note override with defaults
func (c *Config) mergeWithFlags(flagConfig *Config) *Config {

	if isFlagPassed("register") {
		c.ShouldRegister = flagConfig.ShouldRegister
	}
	if isFlagPassed("production") {
		c.IsProduction = flagConfig.IsProduction
	}
	if isFlagPassed("ethprivateKey") {
		c.EthPrivateKey = flagConfig.EthPrivateKey
	}
	if isFlagPassed("ipAddress") {
		c.ProvidedIPAddress = flagConfig.ProvidedIPAddress
	}
	if isFlagPassed("cpuProfile") {
		c.CPUProfileToFile = flagConfig.CPUProfileToFile
	}
	if isFlagPassed("ethConnection") {
		c.EthConnection = flagConfig.EthConnection
	}
	if isFlagPassed("nodeListAddress") {
		c.NodeListAddress = flagConfig.NodeListAddress
	}
	if isFlagPassed("basePath") {
		c.BasePath = flagConfig.BasePath
	}

	return c
}

// createConfigWithFlags edits a config with flags parsed in.
// NOTE: It will note override with defaults
func (c *Config) createConfigWithFlags() string {
	register := flag.Bool("register", true, "defaults to true")
	production := flag.Bool("production", false, "defaults to false")
	ethPrivateKey := flag.String("ethprivateKey", "", "provide private key here to run node on")
	ipAddress := flag.String("ipAddress", "", "specified IPAdress, necessary for running in an internal env e.g. docker")
	cpuProfile := flag.String("cpuProfile", "", "write cpu profile to file")
	ethConnection := flag.String("ethConnection", "", "ethereum endpoint")
	nodeListAddress := flag.String("nodeListAddress", "", "node list address on ethereum")
	basePath := flag.String("basePath", "/.torus", "basePath for Torus node artifacts")
	configPath := flag.String("configPath", "", "override configPath")
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

	return *configPath
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
	flagConf := defaultConfigSettings()

	// NOTE(TO_REMOVE): This was only used in MainServerAddress anyway..
	// nodeIP, err := findExternalIP()
	// if err != nil {
	// 	// QUESTION(TEAM) - unhandled error, was only fmt.Printlnd
	// 	logging.Errorf("%s", err)
	// }

	providedCF := flagConf.createConfigWithFlags()
	if providedCF != "" {
		logging.Infof("overriding configPath to: %s", providedCF)
		configPath = providedCF
	}

	err := readAndMarshallJSONConfig(configPath, &conf)
	if err != nil {
		logging.Warningf("failed to read JSON config with err: %s", err)
	}

	err = env.Parse(&conf)
	if err != nil {
		logging.Error(err.Error())
	}

	conf.mergeWithFlags(&flagConf)

	logging.SetLevelString(conf.LogLevel)

	// TEAM: If you wantr to use localhost just explicitly pass it as an env / flag...
	// if !conf.IsProduction {
	// 	conf.MainServerAddress = "localhost" + ":" + conf.HttpServerPort
	// }
	// retrieve map[string]interface{}

	if conf.ProvidedIPAddress != "" {
		logging.Infof("Running on Specified IP Address: %s", conf.ProvidedIPAddress)
		conf.MainServerAddress = conf.ProvidedIPAddress + ":" + conf.HttpServerPort
	}

	logging.Infof("Final Configuration: %s", conf)

	return &conf
}

func defaultConfigSettings() Config {
	return Config{
		HttpServerPort:             "443",
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
		ServerCert:                 "/.torus/openssl/server.crt",
		ServerKey:                  "/.torus/openssl/server.key",
	}
}
