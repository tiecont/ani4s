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
	host := os.Getenv("REDIS_HOST")
	port := os.Getenv("REDIS_PORT")

	addr := fmt.Sprintf("%s:%s", host, port)

	RDB = redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: "",
		DB:       0,
	})

	pong, err := RDB.Ping(Ctx).Result()
	if err != nil {
		fmt.Println("failed to connect to Redis: %w", err)
		return nil, nil
	}

	fmt.Println("Redis connected:", pong)
	return RDB, Ctx
}
