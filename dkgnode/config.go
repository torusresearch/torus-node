package dkgnode

/* All useful imports */
import (
	"github.com/micro/go-config"
	"github.com/micro/go-config/source/file"
)

type Config struct {
	MyPort           string `json:"myport"`
	EthConnection    string `json:"ethconnection"`
	EthPrivateKey    string `json:"ethprivatekey"`
	BftURI           string `json:"bfturi`
	ABCIServer       string `json:"abciserver`
	P2PListenAddress string `json"p2plistenaddress"`
	NodeListAddress  string `json:"nodelistaddress"`
	HostName         string `json:"hostname"`
	NumberOfNodes    int    `json:"numberofnodes"`
	Threshold        int    `json:"threshold"`
	KeysPerEpoch     int    `json:"keysperepoch"`
}

func loadConfig(suite *Suite, path string) {
	/* Load Config */
	config.Load(file.NewSource(
		file.WithPath(path),
	))
	// retrieve map[string]interface{}
	var conf Config
	config.Scan(&conf)

	suite.Config = &conf
}

func defaultConfigSettings() Config {
	return Config{
		MyPort:           "8000",
		EthConnection:    "https://ropsten.infura.io/v3/1cd8ab320edc46dd81f09a048dca1e50",
		EthPrivateKey:    "29909a750dc6abc3e3c83de9c6da9d6faf9fde4eebb61fa21221415557de5a0b",
		BftURI:           "tcp://0.0.0.0:26657",
		ABCIServer:       "tcp://0.0.0.0:8010",
		P2PListenAddress: "tcp://0.0.0.0:26656",
		NodeListAddress:  "0xd44f7724b0a0800e41283e97be5ec9e875f59811",
		HostName:         "dkg1.tetrator.us",
		NumberOfNodes:    5,
		Threshold:        3,
		KeysPerEpoch:     10,
	}
}
