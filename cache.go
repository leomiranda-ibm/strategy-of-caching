package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/redis/go-redis/v9"
)

type CacheManager struct {
	rdb                *redis.Client
	revalidateInterval time.Duration
}

func (c CacheManager) Set(ctx context.Context, key string, value any, expiration time.Duration) error {
	data := CacheData{
		Data:         value,
		RevalidateAt: time.Now().Add(c.revalidateInterval),
	}

	byteData, err := json.Marshal(data)
	if err != nil {
		return err
	}

	return c.rdb.Set(ctx, key, byteData, expiration).Err()
}

func (c CacheManager) Get(ctx context.Context, key string, expiration time.Duration, v any, revalidateFunc func() (any, error)) error {

	cmd := c.rdb.Get(ctx, key)
	if err := cmd.Err(); err != nil {
		return err
	}

	var data CacheData
	err := json.Unmarshal([]byte(cmd.Val()), &data)
	if err != nil {
		return err
	}

	byteResult, err := json.Marshal(data.Data)
	if err != nil {
		return err
	}

	err = json.Unmarshal(byteResult, v)
	if err != nil {
		return err
	}

	if revalidateFunc != nil && data.RevalidateAt.Before(time.Now()) {
		go c.revalidate(key, expiration, revalidateFunc)
	}

	return nil
}

func makeRevalidateKey(key string) string {
	return fmt.Sprintf("revalidate-%v", key)
}

const maxRevalidating = 1 * time.Minute

func (c CacheManager) revalidate(key string, expiration time.Duration, revalidateFunc func() (any, error)) {
	var (
		ctx           = context.Background()
		revalidateKey = makeRevalidateKey(key)
	)

	// check if any routine is running to do the same thing
	if err := c.rdb.Get(ctx, revalidateKey).Err(); err == nil {
		return
	}

	log.Println("revalidating!!")

	// set revalidating
	c.rdb.Set(ctx, revalidateKey, true, maxRevalidating).Err()

	results, err := revalidateFunc()
	if err != nil {
		c.Del(ctx, key)
		return
	}

	log.Println("finish revalidation")
	_ = c.Set(ctx, key, results, expiration)
	_ = c.rdb.Del(ctx, revalidateKey)
}

func (c CacheManager) Del(ctx context.Context, key string) error {
	return c.rdb.Del(ctx, key).Err()
}

type CacheData struct {
	Data         any
	RevalidateAt time.Time
}
