package dkgnode

/* All useful imports */
import (
	"flag"

	"github.com/micro/go-config"
	"github.com/micro/go-config/source/env"
	"github.com/micro/go-config/source/file"
	cflag "github.com/micro/go-config/source/flag"
	"github.com/torusresearch/torus-public/logging"
)

const DefaultConfigPath = "~/.torus/config.json"

type Config struct {
	// QUESTION(TEAM): I think struct tags should be either camelCase, or the_traditional_one
	// but definitely standardized, and in the case of go-config, i think it's better if we use camelCase
	MyPort                     string `json:"myport"`
	MainServerAddress          string `json:"mainserveraddress"`
	EthConnection              string `json:"ethconnection"`
	EthPrivateKey              string `json:"ethprivatekey"`
	BftURI                     string `json:"bfturi"`
	ABCIServer                 string `json:"abciserver"`
	P2PListenAddress           string `json:"p2plistenaddress"`
	NodeListAddress            string `json:"nodelistaddress"`
	HostName                   string `json:"hostname"`
	NumberOfNodes              int    `json:"numberofnodes"`
	Threshold                  int    `json:"threshold"`
	KeysPerEpoch               int    `json:"keysperepoch"`
	KeyBufferTriggerPercentage int    `json:"keybuffertriggerpercetage"` //percetage threshold of keys left to trigger buffering 90 - 20
	BasePath                   string `json:"buildpath"`

	ShouldRegister    bool   `json:"register"`
	CPUProfileToFile  string `json:"cpuProfile"`
	IsProduction      bool   `json:"production"`
	ProvidedIPAddress string `json:"ipAddress"`
}

func loadConfig() *Config {
	_ = flag.Bool("register", true, "defaults to true")
	_ = flag.Bool("production", false, "defaults to false")
	_ = flag.String("ethprivateKey", "", "provide private key here to run node on")
	_ = flag.String("ipAddress", "", "specified IPAdress, necessary for running in an internal env e.g. docker")
	_ = flag.String("cpuProfile", "", "write cpu profile to file")
	_ = flag.String("ethConnection", "", "ethereum endpoint")
	_ = flag.String("nodeListAddress", "", "node list address on ethereum")

	flagSource := cflag.NewSource()

	conf := defaultConfigSettings()

	nodeIP, err := findExternalIP()
	if err != nil {
		// QUESTION(TEAM) - unhandled error, was only fmt.Printlnd
		logging.Errorf("%s", err)
	}

	// First we have the default settings
	config.Load(
		// Then we override with the config file
		file.NewSource(file.WithPath(DefaultConfigPath)),
		// Then we override with ENV vars.
		env.NewSource(),
		// Then override env with flags
		flagSource,
	)

	config.Scan(&conf)
	// ^^^^^
	//TODO: validation checks on private key

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
		conf.HostName = conf.ProvidedIPAddress
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
		HostName:                   "",
		NumberOfNodes:              5,
		Threshold:                  3,
		KeysPerEpoch:               100,
		KeyBufferTriggerPercentage: 80,
		BasePath:                   "/.torus",
	}
}
