package dkgnode

import (
	"errors"
	"github.com/patrickmn/go-cache"
	"github.com/torusresearch/torus-public/common"
	"github.com/torusresearch/torus-public/db"
	"github.com/torusresearch/torus-public/keygen"
	"math/big"
	// "github.com/torusresearch/torus-public/logging"
	"time"
)

// TorusDB represents a set of methods necessary for the operation of torus node
// NOTE: work in progress
type TorusDB interface {
	// StoreLog(address string, log *ShareLog) error
	// GetLog(address string) (*ShareLog, error)
	// AppendLog ?

	// StoreAddressMapping(address string) error
	// GetAddressMapping(address string) error
	// Modify mapping by index??

	// StoreShareIndexMapping() error
	// GetShareIndexMapping() error

	// StoreSecretMapping() error
	// GetSecretMapping() error

	RetrieveCompletedShare(keyIndex big.Int) (Si *big.Int, Siprime *big.Int, PublicKey *common.Point, err error)

	keygen.AVSSKeygenStorage
}

type DBSuite struct {
	Instance TorusDB
}

func SetupDB(suite *Suite) error {
	torusLdb, err := db.NewTorusLDB(suite.Config.BasePath)
	if err != nil {
		return errors.New("Was not able to start leveldb: " + err.Error())
	}
	suite.DBSuite = &DBSuite{Instance: torusLdb}
	return nil
}

// CacheSuite - handles caching
type CacheSuite struct {
	CacheInstance *cache.Cache
	TokenCaches   map[string]*cache.Cache
}

// SetupCache - set up caching for handling tokens and secrets
func SetupCache(suite *Suite) {
	// Create a cache with a default expiration time of no expiration time, and which
	// purges expired items every 10 minutes
	secretCache := cache.New(cache.NoExpiration, 10*time.Minute)
	tokenCache := make(map[string]*cache.Cache)
	// TODO: UNUSED!
	// oauthCache := cache.New(60*time.Second, 1*time.Minute)

	suite.CacheSuite = &CacheSuite{
		secretCache,
		tokenCache,
	}

}
