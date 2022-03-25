package config

import (
	"github.com/go-redis/redis"
	log "github.com/sirupsen/logrus"
	"os"
)

var (
	redisClient *redis.Client
)

func NewRedis() {
	redisURL := os.Getenv(EnvRedisUrl)
	opt, err := redis.ParseURL(redisURL)
	if err != nil {
		panic(err)
	}

	redisClient = redis.NewClient(opt)

	_, err = redisClient.Ping().Result()
	if err != nil {
		panic(err)
	}

	closeFn := func() {
		log.Info("closing redis conn")

		err = redisClient.Close()
		if err != nil {
			log.WithFields(log.Fields{
				"context": "close_redis_conn",
				"method":  "config/close",
			}).Error(err)
		}
	}

	closeFns = append(closeFns, closeFn)

}

func GetRedis() *redis.Client {
	return redisClient
}
