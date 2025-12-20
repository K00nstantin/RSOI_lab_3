package main

import (
	"RSOI_lab_2/pkg/models"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

var db *gorm.DB

func main() {
	log.Println("Starting rating service...")

	host := getEnv("DB_HOST", "postgres")
	port := getEnv("DB_PORT", "5432")
	user := getEnv("DB_USER", "program")
	password := getEnv("DB_PASSWORD", "test")
	dbname := getEnv("DB_NAME", "ratings")

	dsn := fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%s sslmode=disable TimeZone=UTC",
		host, user, password, dbname, port)

	log.Printf("Connecting to database: %s@%s:%s/%s", user, host, port, dbname)

	var err error
	maxRetries := 10
	for i := 0; i < maxRetries; i++ {
		db, err = gorm.Open(postgres.Open(dsn), &gorm.Config{})
		if err == nil {
			break
		}
		log.Printf("Database connection attempt %d/%d failed: %v", i+1, maxRetries, err)
		if i < maxRetries-1 {
			time.Sleep(5 * time.Second)
		}
	}

	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	err = db.AutoMigrate(&models.Rating{})
	if err != nil {
		log.Fatalf("Database migration failed: %v", err)
	}

	log.Println("Database connected successfully")

	sqlDB, err := db.DB()
	if err != nil {
		log.Fatalf("Failed to get database instance: %v", err)
	}

	if err := sqlDB.Ping(); err != nil {
		log.Fatalf("Database ping failed: %v", err)
	}

	log.Println("Database ping successful")

	seedTestData()

	server := gin.Default()
	server.GET("/api/v1/rating", getRating)
	server.PUT("/api/v1/rating", updateRating)
	server.POST("/api/v1/rating/adjust", adjustRating)
	server.GET("/manage/health", healthCheck)

	log.Println("Rating service starting on :8050")
	if err := server.Run(":8050"); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}

func getRating(c *gin.Context) {
	username := c.GetHeader("X-User-Name")
	if username == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "X-User-Name header is required"})
		return
	}
	var rating models.Rating
	err := db.Where("username = ?", username).First(&rating).Error
	if err != nil {
		newrating := models.Rating{
			Username: username,
			Stars:    1,
		}
		db.Create(&newrating)
		c.JSON(http.StatusOK, gin.H{"stars": newrating.Stars})
		return
	}
	c.JSON(http.StatusOK, gin.H{"stars": rating.Stars})
}

func updateRating(c *gin.Context) {
	var request struct {
		Username string `json:"username"`
		Stars    int    `json:"stars"`
	}
	err := c.ShouldBindJSON(&request)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request format"})
		return
	}
	if request.Stars < 1 {
		request.Stars = 1
	}
	if request.Stars > 100 {
		request.Stars = 100
	}
	var rating models.Rating
	query := db.Where("username = ?", request.Username).First(&rating)
	if query.Error == nil {
		rating.Stars = request.Stars
		db.Save(&rating)
	} else {
		rating = models.Rating{
			Username: request.Username,
			Stars:    request.Stars,
		}
		db.Create(&rating)
	}
	c.JSON(http.StatusOK, gin.H{"stars": rating.Stars})
}

func adjustRating(c *gin.Context) {
	var request struct {
		Username string `json:"username" binding:"required"`
		Delta    int    `json:"delta"`
	}
	err := c.ShouldBindJSON(&request)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request format"})
		return
	}

	var rating models.Rating
	err = db.Where("username = ?", request.Username).First(&rating).Error
	if err != nil {
		rating = models.Rating{
			Username: request.Username,
			Stars:    1,
		}
		if err := db.Create(&rating).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create rating"})
			return
		}
	}

	newStars := rating.Stars + request.Delta
	if newStars < 1 {
		newStars = 1
	}
	if newStars > 100 {
		newStars = 100
	}

	rating.Stars = newStars
	if err := db.Save(&rating).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update rating"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"stars": rating.Stars})
}

func seedTestData() {
	testUsers := []models.Rating{
		{Username: "alice", Stars: 75},
		{Username: "bob", Stars: 45},
		{Username: "charlie", Stars: 90},
	}

	for _, user := range testUsers {
		var existing models.Rating
		if err := db.Where("username = ?", user.Username).First(&existing).Error; err != nil {
			db.Create(&user)
		}
	}
	log.Println("Test data seeded")
}

func healthCheck(ctx *gin.Context) {
	sqlDB, err := db.DB()
	if err != nil {
		ctx.JSON(http.StatusServiceUnavailable, gin.H{
			"status":  "DOWN",
			"details": "Database connection failed",
			"error":   err.Error(),
		})
		return
	}
	if err := sqlDB.Ping(); err != nil {
		ctx.JSON(http.StatusServiceUnavailable, gin.H{
			"status":  "DOWN",
			"details": "Database ping failed",
			"error":   err.Error(),
		})
		return
	}
	ctx.JSON(http.StatusOK, gin.H{
		"status":  "UP",
		"details": "Host localhost:8050 is active",
	})
}

func getEnv(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
}
