package db

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/redis/go-redis/v9"
)

var Redis *redis.Client

func ConnectRedis() {
	url := os.Getenv("REDIS_URL")
	if url == "" {
		url = "localhost:6379" // fallback
	}

	client := redis.NewClient(&redis.Options{
		Addr: url,
		DB:   0, // use default DB
	})

	_, err := client.Ping(context.Background()).Result()
	if err != nil {
		log.Fatalf("Unable to connect to Redis: %v", err)
	}

	Redis = client
	fmt.Println("Connected to Redis successfully.")
}
