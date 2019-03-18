package dkgnode

import (
	"time"

	"github.com/patrickmn/go-cache"
)

type CacheSuite struct {
	CacheInstance      *cache.Cache
	OAuthCacheInstance *cache.Cache
}

func SetupCache(suite *Suite) {
	// Create a cache with a default expiration time of no expiration time, and which
	// purges expired items every 10 minutes
	secretCache := cache.New(cache.NoExpiration, 10*time.Minute)
	oauthCache := cache.New(60*time.Second, 1*time.Minute)

	suite.CacheSuite = &CacheSuite{
		secretCache,
		oauthCache,
	}

	secretAssignment := make(map[string]SecretAssignment)
	suite.CacheSuite.CacheInstance.Set("Secret_ASSIGNMENT", secretAssignment, -1)
	suite.CacheSuite.CacheInstance.Set("LAST_ASSIGNED", 0, -1)

}
