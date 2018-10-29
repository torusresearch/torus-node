package dkgnode

/* Al useful imports */
import (
	"github.com/micro/go-config"
	"github.com/micro/go-config/source/file"
)

type Config struct {
	MakeMasterOnError string `json:"makeMasterOnError"`
	ClusterIp         string `json:"clusterip"`
	MyPort            string `json:"myport"`
	EthConnection     string `json:"ethconnection"`
	EthPrivateKey     string `json:"ethprivatekey"`
	NodeListAddress   string `json:"nodelistaddress`
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
