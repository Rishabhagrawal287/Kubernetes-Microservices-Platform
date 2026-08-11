package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/redis/go-redis/v9"
)

// Product is stored as a single JSON blob per ID under key "product:<id>".
// Simplification worth knowing: decrementStock does a read-modify-write,
// which isn't atomic under concurrent orders for the same product. Fine for
// a learning platform's traffic; a real system would use a Lua script or
// Redis hash + HINCRBY instead. Revisit in the hardening phase if it matters.
type Product struct {
	ID    string  `json:"id"`
	Name  string  `json:"name"`
	Price float64 `json:"price"`
	Stock int     `json:"stock"`
}

var ctx = context.Background()

func newRedisClient() *redis.Client {
	addr := os.Getenv("REDIS_ADDR")
	if addr == "" {
		addr = "localhost:6379"
	}
	return redis.NewClient(&redis.Options{Addr: addr})
}

func productKey(id string) string {
	return fmt.Sprintf("product:%s", id)
}

func saveProduct(rdb *redis.Client, p Product) error {
	data, err := json.Marshal(p)
	if err != nil {
		return err
	}
	if err := rdb.Set(ctx, productKey(p.ID), data, 0).Err(); err != nil {
		return err
	}
	productStockLevel.WithLabelValues(p.ID).Set(float64(p.Stock))
	return nil
}

func getProduct(rdb *redis.Client, id string) (*Product, error) {
	data, err := rdb.Get(ctx, productKey(id)).Bytes()
	if err != nil {
		return nil, err
	}
	var p Product
	if err := json.Unmarshal(data, &p); err != nil {
		return nil, err
	}
	return &p, nil
}

func listProducts(rdb *redis.Client) ([]Product, error) {
	keys, err := rdb.Keys(ctx, "product:*").Result()
	if err != nil {
		return nil, err
	}
	products := make([]Product, 0, len(keys))
	for _, k := range keys {
		data, err := rdb.Get(ctx, k).Bytes()
		if err != nil {
			continue
		}
		var p Product
		if err := json.Unmarshal(data, &p); err == nil {
			products = append(products, p)
		}
	}
	return products, nil
}

func decrementStock(rdb *redis.Client, id string, qty int) error {
	p, err := getProduct(rdb, id)
	if err != nil {
		return err
	}
	p.Stock -= qty
	if p.Stock < 0 {
		p.Stock = 0
	}
	return saveProduct(rdb, *p)
}
