package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/redis/go-redis/v9"
)

const userCacheKey = "users"
const maxTimeExp = 5 * time.Minute

func main() {
	rdb := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})
	defer rdb.Close()

	c := CacheManager{
		rdb:                rdb,
		revalidateInterval: 2 * time.Second,
	}

	c.Del(context.Background(), userCacheKey)

	http.HandleFunc("/users", func(w http.ResponseWriter, r *http.Request) {
		var (
			ctx   = r.Context()
			users []User
		)

		revalidateFunc := func() (any, error) {
			return getFromDataBase()
		}

		if err := c.Get(ctx, userCacheKey, maxTimeExp, &users, revalidateFunc); err == nil {
			log.Println("getting from cache")
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(users)
			return
		}

		log.Println("getting from database")
		users, err := getFromDataBase()
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		_ = c.Set(ctx, userCacheKey, users, maxTimeExp)

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(users)
	})

	println("listening on http://localhost:8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}

type User struct {
	Name      string `json:"name"`
	CreatedAt int    `json:"createdAt"`
}

func getFromDataBase() ([]User, error) {
	time.Sleep(1 * time.Second)

	return []User{
		{
			Name:      "Leo",
			CreatedAt: time.Now().Second(),
		},
	}, nil
}
