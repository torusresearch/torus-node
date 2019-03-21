package dkgnode

import (
	"time"

	"github.com/patrickmn/go-cache"
)

// TorusDB represents a set of methods necessary for the operation of torus node
// NOTE: work in progress
type TorusDB interface {
	StoreLog(address string, log *ShareLog) error
	GetLog(address string) (*ShareLog, error)
	// AppendLog ?

	StoreAddressMapping(address string) error
	GetAddressMapping(address string) error
	// Modify mapping by index??

	StoreShareIndexMapping() error
	GetShareIndexMapping() error

	StoreSecretMapping() error
	GetSecretMapping() error
}

type CacheSuite struct {
	CacheInstance *cache.Cache
	// TODO(TEAM) - Not used, shouldn't it be removed?
	// OAuthCacheInstance *cache.Cache
}

func SetUpCache(suite *Suite) {
	// Create a cache with a default expiration time of no expiration time, and which
	// purges expired items every 10 minutes
	secretCache := cache.New(cache.NoExpiration, 10*time.Minute)

	// TODO: UNUSED!
	// oauthCache := cache.New(60*time.Second, 1*time.Minute)

	suite.CacheSuite = &CacheSuite{
		secretCache,
		// TODO: UNUSED!
		// oauthCache,
	}

	// TODO(TEAM) - these are literally unused anywhere, should they still be here??
	// secretAssignment := make(map[string]SecretAssignment)
	//
	// suite.CacheSuite.CacheInstance.Set("Secret_ASSIGNMENT", secretAssignment, -1)
	// suite.CacheSuite.CacheInstance.Set("LAST_ASSIGNED", 0, -1)

}
