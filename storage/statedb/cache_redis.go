// Copyright 2020 The klaytn Authors
// This file is part of the klaytn library.
//
// The klaytn library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The klaytn library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the klaytn library. If not, see <http://www.gnu.org/licenses/>.

package statedb

import (
	"errors"
	"time"

	"github.com/go-redis/redis/v7"
	"github.com/klaytn/klaytn/common/hexutil"
)

const (
	redisSubscriptionChannelSize  = 100 // default value of redis client
	redisSubscriptionChannelBlock = "latestBlock"
)

var (
	redisCacheDialTimeout = time.Duration(900 * time.Millisecond)
	redisCacheTimeout     = time.Duration(900 * time.Millisecond)

	errRedisNoEndpoint = errors.New("redis endpoint not specified")
)

type RedisCache struct {
	client redis.UniversalClient
	pubSub *redis.PubSub
}

func newRedisClient(endpoints []string, isCluster bool) (redis.UniversalClient, error) {
	if endpoints == nil {
		return nil, errRedisNoEndpoint
	}

	// cluster-enabled redis can have more than one shard
	if isCluster {
		return redis.NewClusterClient(&redis.ClusterOptions{
			// it takes Timeout * (MaxRetries+1) to raise an error
			Addrs:        endpoints,
			DialTimeout:  redisCacheDialTimeout,
			ReadTimeout:  redisCacheTimeout,
			WriteTimeout: redisCacheTimeout,
			MaxRetries:   2,
		}), nil
	}

	return redis.NewClient(&redis.Options{
		// it takes Timeout * (MaxRetries+1) to raise an error
		Addr:         endpoints[0],
		DialTimeout:  redisCacheDialTimeout,
		ReadTimeout:  redisCacheTimeout,
		WriteTimeout: redisCacheTimeout,
		MaxRetries:   2,
	}), nil
}

func NewRedisCache(config *TrieNodeCacheConfig) (*RedisCache, error) {
	cli, err := newRedisClient(config.RedisEndpoints, config.RedisClusterEnable)
	if err != nil {
		logger.Error("failed to create a redis client", "err", err, "endpoint", config.RedisEndpoints,
			"isCluster", config.RedisClusterEnable)
		return nil, err
	}

	logger.Info("Initialize trie node cache with redis", "endpoint", config.RedisEndpoints,
		"isCluster", config.RedisClusterEnable)
	return &RedisCache{client: cli, pubSub: nil}, nil
}

func (cache *RedisCache) Get(k []byte) []byte {
	val, err := cache.client.Get(hexutil.Encode(k)).Bytes()
	if err != nil {
		// TODO-Klyatn: Print specific errors if needed
		logger.Debug("cannot get an item from redis cache", "err", err, "key", hexutil.Encode(k))
		return nil
	}
	return val
}

func (cache *RedisCache) Set(k, v []byte) {
	if err := cache.client.Set(hexutil.Encode(k), v, 0).Err(); err != nil {
		logger.Warn("failed to set an item on redis cache", "err", err, "key", hexutil.Encode(k))
	}
}

func (cache *RedisCache) Has(k []byte) ([]byte, bool) {
	val := cache.Get(k)
	if val == nil {
		return nil, false
	}
	return val, true
}

func (cache *RedisCache) publish(channel string, msg string) error {
	return cache.client.Publish(channel, msg).Err()
}

// subscribe subscribes the redis client to the given channel.
// It returns an existing *redis.PubSub subscribing previously registered channels also.
func (cache *RedisCache) subscribe(channel string) *redis.PubSub {
	if cache.pubSub == nil {
		cache.pubSub = cache.client.Subscribe()
	}
	if err := cache.pubSub.Subscribe(channel); err != nil {
		logger.Error("failed to subscribe channel", "err", err, "channel", channel)
	}
	return cache.pubSub
}

func (cache *RedisCache) PublishBlock(msg string) error {
	return cache.publish(redisSubscriptionChannelBlock, msg)
}

func (cache *RedisCache) SubscribeBlockCh() <-chan *redis.Message {
	return cache.subscribe(redisSubscriptionChannelBlock).ChannelSize(redisSubscriptionChannelSize)
}

func (cache *RedisCache) UnsubscribeBlock() error {
	if cache.pubSub == nil {
		return nil
	}
	return cache.pubSub.Unsubscribe(redisSubscriptionChannelBlock)
}

func (cache *RedisCache) UpdateStats() interface{} {
	return nil
}

func (cache *RedisCache) SaveToFile(filePath string, concurrency int) error {
	return nil
}

func (cache *RedisCache) Close() error {
	return cache.client.Close()
}
