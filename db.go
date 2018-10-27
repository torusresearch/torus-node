package main

import (
	"time"

	"github.com/patrickmn/go-cache"
)

type CacheSuite struct {
	CacheInstance *cache.Cache
}

func setUpCache(suite *Suite) {
	// Create a cache with a default expiration time of 5 minutes, and which
	// purges expired items every 10 minutes
	c := cache.New(cache.NoExpiration, 10*time.Minute)
	cacheSuite := CacheSuite{c}
	suite.CacheSuite = &cacheSuite
}
