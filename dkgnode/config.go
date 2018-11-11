package dkgnode

/* Al useful imports */
import (
	"github.com/micro/go-config"
	"github.com/micro/go-config/source/file"
)

type Config struct {
	MakeMasterOnError string `json:"makemasteronerror"`
	MyPort            string `json:"myport"`
	EthConnection     string `json:"ethconnection"`
	EthPrivateKey     string `json:"ethprivatekey"`
	NodeListAddress   string `json:"nodelistaddress"`
	HostName          string `json:"hostname"`
	NumberOfNodes     int    `json:"numberofnodes"`
	Threshold         int    `json:"threshold"`
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
