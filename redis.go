package RedisStateStore

import (
	"fmt"

	faasflow "github.com/faasflow/sdk"
	redis "github.com/go-redis/redis"
)

type RedisStateStore struct {
	KeyPath    string
	rds        redis.UniversalClient
	RetryCount int
}

// Update Compare and Update a valuer
type Incrementer interface {
	Incr(key string, value int64) (int64, error)
}

func GetRedisStateStore(redisUri string) (faasflow.StateStore, error) {
	stateStore := &RedisStateStore{}

	client := redis.NewClient(&redis.Options{
		Addr: redisUri,
	})

	err := client.Ping().Err()
	if err != nil {
		return nil, err
	}

	stateStore.rds = client
	return stateStore, nil
}

// Configure
func (rss *RedisStateStore) Configure(flowName string, requestId string) {
	rss.KeyPath = fmt.Sprintf("faasflow.%s.%s", flowName, requestId)
}

// Init (Called only once in a request)
func (rss *RedisStateStore) Init() error {
	return nil
}

// Update Compare and Update a valuer
func (rss *RedisStateStore) Update(key string, oldValue string, newValue string) error {
	key = rss.KeyPath + "." + key
	client := rss.rds
	script :=  redis.NewScript(`
		if redis.call('GET', KEYS[1]) == ARGV[1] then
			redis.call('SET', KEYS[1], ARGV[2]);
			return 0;
		end
		return 1;
	`)
	cmd := script.Run(client, []string{key}, oldValue, newValue)
	if cmd.Err() != nil {
		return fmt.Errorf("failed to set key %s, error %v", key, cmd.Err())
	}
	success, err := cmd.Int()
	if err != nil {
		return fmt.Errorf("failed to set key %s, error %v", key, err)
	}
	if success != 0 {
		return fmt.Errorf("failed to set key %s, error %v", key, err)
	}
	return nil
}

// Update Compare and Update a valuer
func (rss *RedisStateStore) Incr(key string, value int64) (int64, error) {
	key = rss.KeyPath + "." + key
	client := rss.rds
	return client.IncrBy(key, value).Result()
}

// Set Sets a value (override existing, or create one)
func (rss *RedisStateStore) Set(key string, value string) error {
	key = rss.KeyPath + "." + key
	client := rss.rds
	err := client.Set(key, value, 0).Err()
	if err != nil {
		return fmt.Errorf("failed to set key %s, error %v", key, err)
	}
	return nil
}

// Get Gets a value
func (rss *RedisStateStore) Get(key string) (string, error) {
	key = rss.KeyPath + "." + key
	client := rss.rds
	value, err := client.Get(key).Result()
	if err == redis.Nil {
		return "", fmt.Errorf("failed to get key %s, nil", key)
	} else if err != nil {
		return "", fmt.Errorf("failed to get key %s, %v", key, err)
	}

	return value, nil
}

// Cleanup (Called only once in a request)
func (rss *RedisStateStore) Cleanup() error {
	key := rss.KeyPath + ".*"
	client := rss.rds
	var rerr error

	iter := client.Scan(0, key, 0).Iterator()
	for iter.Next() {
		err := client.Del(iter.Val()).Err()
		if err != nil {
			rerr = err
		}
	}

	if err := iter.Err(); err != nil {
		rerr = err
	}
	return rerr
}
