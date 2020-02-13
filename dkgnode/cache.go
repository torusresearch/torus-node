package dkgnode

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/patrickmn/go-cache"
	"github.com/torusresearch/torus-common/common"
	pcmn "github.com/torusresearch/torus-node/common"
	"github.com/torusresearch/torus-node/eventbus"
	"github.com/torusresearch/torus-node/telemetry"
)

type CacheSuiteSyncMap struct {
	sync.Map
}

func (s *CacheSuiteSyncMap) Get(key string) *cache.Cache {
	valueInterface, ok := s.Map.Load(key)
	if !ok {
		return nil
	}

	value, ok := valueInterface.(*cache.Cache)
	if !ok {
		return nil
	}

	return value
}

func (s *CacheSuiteSyncMap) Set(key string, value *cache.Cache) {
	s.Map.Store(key, value)
}
func (s *CacheSuiteSyncMap) Exists(key string) (ok bool) {
	_, ok = s.Map.Load(key)
	return
}

func NewCacheService(ctx context.Context, eventBus eventbus.Bus) *BaseService {
	cacheCtx, cancel := context.WithCancel(context.WithValue(ctx, ContextID, "cache"))
	cacheService := CacheService{
		cancel:        cancel,
		parentContext: ctx,
		context:       cacheCtx,
		eventBus:      eventBus,
	}
	cacheService.serviceLibrary = NewServiceLibrary(cacheService.eventBus, cacheService.Name())
	return NewBaseService(&cacheService)
}

type CacheService struct {
	tokenCache  *CacheSuiteSyncMap
	signerCache *cache.Cache

	bs             *BaseService
	cancel         context.CancelFunc
	parentContext  context.Context
	context        context.Context
	eventBus       eventbus.Bus
	serviceLibrary ServiceLibrary
}

func (c *CacheService) Name() string {
	return "cache"
}

func (c *CacheService) OnStart() error {
	c.tokenCache = &CacheSuiteSyncMap{}
	c.signerCache = cache.New(cache.NoExpiration, time.Minute)
	return nil
}

func (c *CacheService) OnStop() error {
	return nil
}

func (c *CacheService) Call(method string, args ...interface{}) (interface{}, error) {
	telemetry.IncrementCounter(pcmn.TelemetryConstants.Generic.TotalServiceCalls, pcmn.TelemetryConstants.Cache.Prefix)
	switch method {
	// TokenCommitExists(verifier string, tokenCommitment string) (exists bool)
	case "token_commit_exists":
		telemetry.IncrementCounter(pcmn.TelemetryConstants.Cache.TokenCommitExistsCounter, pcmn.TelemetryConstants.Cache.Prefix)

		var args0, args1 string
		_ = castOrUnmarshal(args[0], &args0)
		_ = castOrUnmarshal(args[1], &args1)

		exists := c.tokenCommitExists(args0, args1)
		return exists, nil
	// GetTokenCommitKey(verifier string, tokenCommitment string) (pubKey common.Point)
	case "get_token_commit_key":
		telemetry.IncrementCounter(pcmn.TelemetryConstants.Cache.GetTokenCommitKeyCounter, pcmn.TelemetryConstants.Cache.Prefix)

		var args0, args1 string
		_ = castOrUnmarshal(args[0], &args0)
		_ = castOrUnmarshal(args[1], &args1)

		pubKey := c.getTokenCommitKey(args0, args1)
		return pubKey, nil
	// RecordTokenCommit(verifier string, tokenCommitment string, pubKey common.Point)
	case "record_token_commit":
		telemetry.IncrementCounter(pcmn.TelemetryConstants.Cache.RecordTokenCommitCounter, pcmn.TelemetryConstants.Cache.Prefix)

		var args0, args1 string
		var args2 common.Point
		_ = castOrUnmarshal(args[0], &args0)
		_ = castOrUnmarshal(args[1], &args1)
		_ = castOrUnmarshal(args[2], &args2)

		c.recordTokenCommit(args0, args1, args2)
		return nil, nil
	// SignerSigExists(signature string) (exists bool)
	case "signer_sig_exists":
		telemetry.IncrementCounter(pcmn.TelemetryConstants.Cache.SignerSigExistsCounter, pcmn.TelemetryConstants.Cache.Prefix)

		var args0 string
		_ = castOrUnmarshal(args[0], &args0)
		exists := c.signerSigExists(args0)
		return exists, nil
	// RecordSignerSig(signature string)
	case "record_signer_sig":
		telemetry.IncrementCounter(pcmn.TelemetryConstants.Cache.RecordSignerSigCounter, pcmn.TelemetryConstants.Cache.Prefix)

		var args0 string
		_ = castOrUnmarshal(args[0], &args0)
		return nil, c.recordSignerSig(args0)
	}
	return nil, fmt.Errorf("cache service method %v not found", method)
}

func (c *CacheService) SetBaseService(bs *BaseService) {
	c.bs = bs
}

func (c *CacheService) signerSigExists(signature string) (exists bool) {
	_, exists = c.signerCache.Get(signature)
	return
}

func (c *CacheService) recordSignerSig(signature string) error {
	return c.signerCache.Add(signature, true, time.Minute)
}

// tokenCommitExists - checks if a token exists in cache for given verifier and tokenCommitment
// creates a new verifier cache if it does not exist
func (c *CacheService) tokenCommitExists(verifier string, tokenCommitment string) (exists bool) {
	if !c.tokenCache.Exists(verifier) {
		c.tokenCache.Set(verifier, cache.New(cache.NoExpiration, 10*time.Minute))
	}
	_, found := c.tokenCache.Get(verifier).Get(tokenCommitment)
	return found
}

func (c *CacheService) getTokenCommitKey(verifier string, tokenCommitment string) (pubKey common.Point) {
	if !c.tokenCache.Exists(verifier) {
		c.tokenCache.Set(verifier, cache.New(cache.NoExpiration, 10*time.Minute))
	}
	item, found := c.tokenCache.Get(verifier).Get(tokenCommitment)
	if found {
		tokenCommitmentData := item.(TokenCommitmentData)
		return tokenCommitmentData.PubKey
	}
	return common.Point{}
}

func (c *CacheService) recordTokenCommit(verifier string, tokenCommitment string, pubKey common.Point) {
	if !c.tokenCache.Exists(verifier) {
		c.tokenCache.Set(verifier, cache.New(cache.NoExpiration, 10*time.Minute))
	}
	tokenCommitmentData := TokenCommitmentData{Exists: true, PubKey: pubKey}
	c.tokenCache.Get(verifier).Set(tokenCommitment, tokenCommitmentData, 90*time.Minute)
}
