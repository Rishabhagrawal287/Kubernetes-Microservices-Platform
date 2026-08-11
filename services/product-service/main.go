package main

import (
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	rdb := newRedisClient()

	go startOrderEventsConsumer(rdb)

	r := gin.Default()
	r.Use(metricsMiddleware())

	// Liveness: is the process up at all
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok", "service": "product-service"})
	})

	// Readiness: only ready once Redis actually responds
	r.GET("/ready", func(c *gin.Context) {
		if err := rdb.Ping(ctx).Err(); err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"status": "not-ready", "service": "product-service"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "ready", "service": "product-service"})
	})

	// Scraped by Prometheus via the ServiceMonitor in helm/product-service
	r.GET("/metrics", metricsHandler())

	r.GET("/api/products", func(c *gin.Context) {
		products, err := listProducts(rdb)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, products)
	})

	r.GET("/api/products/:id", func(c *gin.Context) {
		p, err := getProduct(rdb, c.Param("id"))
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "product not found"})
			return
		}
		c.JSON(http.StatusOK, p)
	})

	r.POST("/api/products", func(c *gin.Context) {
		var p Product
		if err := c.ShouldBindJSON(&p); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if p.ID == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "id is required"})
			return
		}
		if err := saveProduct(rdb, p); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusCreated, p)
	})

	r.Run(":" + port)
}
