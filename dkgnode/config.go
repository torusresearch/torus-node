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
	NodeListAddress  string `json:"nodelistaddress"`
	HostName         string `json:"hostname"`
	NumberOfNodes    int    `json:"numberofnodes"`
	Threshold        int    `json:"threshold"`
	ABCIServer       string `json:"abciserver`
	P2PListenAddress string `json"p2plistenaddress"`
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
