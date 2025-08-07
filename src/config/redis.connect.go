package config

import (
	"context"
	"fmt"
	"os"
	_ "time"

	"github.com/redis/go-redis/v9"
)

var (
	RDB *redis.Client
	Ctx = context.Background()
)

func ConnectRedis() (*redis.Client, context.Context) {
	mode := os.Getenv("REDIS_MODE")

	if mode == "sentinel" {
		// Redis Sentinel Mode
		masterName := os.Getenv("REDIS_MASTER_NAME")
		password := os.Getenv("REDIS_PASSWORD")
		sentinels := []string{
			"redis-sentinel-node-0.redis-sentinel-headless.architecture.svc.cluster.local:26379",
			"redis-sentinel-node-1.redis-sentinel-headless.architecture.svc.cluster.local:26379",
			"redis-sentinel-node-2.redis-sentinel-headless.architecture.svc.cluster.local:26379",
			"redis-sentinel-node-3.redis-sentinel-headless.architecture.svc.cluster.local:26379",
			"redis-sentinel-node-4.redis-sentinel-headless.architecture.svc.cluster.local:26379",
		}

		RDB = redis.NewFailoverClient(&redis.FailoverOptions{
			MasterName:       masterName,
			SentinelAddrs:    sentinels,
			Password:         password,
			SentinelPassword: password,
			DB:               0,
		})
	} else {
		// Standalone Redis Mode (local dev)
		host := os.Getenv("REDIS_HOST")
		port := os.Getenv("REDIS_PORT")
		password := os.Getenv("REDIS_PASSWORD")
		addr := fmt.Sprintf("%s:%s", host, port)

		RDB = redis.NewClient(&redis.Options{
			Addr:     addr,
			Password: password,
			DB:       0,
		})
	}

	pong, err := RDB.Ping(Ctx).Result()
	if err != nil {
		fmt.Printf("Failed to connect to Redis (%s mode): %v\n", mode, err)
		return nil, nil
	}

	fmt.Println("Redis connected (mode:", mode, "):", pong)
	return RDB, Ctx
}
