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
