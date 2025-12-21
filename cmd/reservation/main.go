package main

import (
	"RSOI_lab_3/pkg/models"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

var db *gorm.DB

func main() {
	log.Println("Starting reservation service...")

	host := getEnv("DB_HOST", "postgres")
	port := getEnv("DB_PORT", "5432")
	user := getEnv("DB_USER", "program")
	password := getEnv("DB_PASSWORD", "test")
	dbname := getEnv("DB_NAME", "reservations")

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

	err = db.AutoMigrate(&models.Reservation{})
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
	server.GET("/api/v1/reservations", getReservations)
	server.GET("/api/v1/reservations/active/count", getActiveReservationsCount)
	server.POST("/api/v1/reservations", createReservations)
	server.POST("/api/v1/reservations/:reservationUid/return", returnBook)
	server.GET("/manage/health", healthCheck)

	log.Println("Reservation service starting on :8070")
	if err := server.Run(":8070"); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}

func getReservations(c *gin.Context) {
	username := c.GetHeader("X-User-Name")
	if username == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "X-User-Name header is required"})
		return
	}

	var reservations []models.Reservation
	err := db.Where("username = ?", username).Find(&reservations).Error
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	items := make([]gin.H, len(reservations))
	for i, res := range reservations {
		items[i] = gin.H{
			"reservationUid": res.ReservationUid,
			"status":         res.Status,
			"startDate":      res.StartDate.Format("2006-01-02"),
			"tillDate":       res.TillDate.Format("2006-01-02"),
			"bookUid":        res.BookUid,
			"libraryUid":     res.LibraryUid,
			"bookCondition":  res.BookCondition,
		}
	}

	c.JSON(http.StatusOK, items)
}

func getActiveReservationsCount(c *gin.Context) {
	username := c.GetHeader("X-User-Name")
	if username == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "X-User-Name header is required"})
		return
	}

	var count int64
	err := db.Model(&models.Reservation{}).
		Where("username = ? AND status = ?", username, "RENTED").
		Count(&count).Error
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"count": count})
}

func createReservations(c *gin.Context) {
	username := c.GetHeader("X-User-Name")
	if username == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "X-User-Name header is required"})
		return
	}
	var request struct {
		BookUid       string `json:"bookUid" binding:"required"`
		LibraryUid    string `json:"libraryUid" binding:"required"`
		TillDate      string `json:"tillDate" binding:"required"`
		BookCondition string `json:"bookCondition"`
	}
	err := c.ShouldBindJSON(&request)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid request",
			"details": err.Error(),
		})
		return
	}
	tillDate, err := time.Parse("2006-01-02", request.TillDate)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid data format"})
		return
	}
	reservation := models.Reservation{
		ReservationUid: uuid.New().String(),
		Username:       username,
		BookUid:        request.BookUid,
		LibraryUid:     request.LibraryUid,
		Status:         "RENTED",
		BookCondition:  request.BookCondition,
		StartDate:      time.Now(),
		TillDate:       tillDate,
	}
	err = db.Create(&reservation).Error
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create reservation"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"reservationUid": reservation.ReservationUid,
		"status":         reservation.Status,
		"startDate":      reservation.StartDate.Format("2006-01-02"),
		"tillDate":       reservation.TillDate.Format("2006-01-02"),
		"bookUid":        reservation.BookUid,
		"libraryUid":     reservation.LibraryUid,
	})
}

func returnBook(c *gin.Context) {
	username := c.GetHeader("X-User-Name")
	if username == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "X-User-Name header is required"})
		return
	}
	reservationUid := c.Param("reservationUid")

	var request struct {
		Condition string `json:"condition" binding:"required"`
		Date      string `json:"date" binding:"required"`
		Status    string `json:"status"`
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request", "details": err.Error()})
		return
	}
	if request.Condition != "EXCELLENT" && request.Condition != "GOOD" && request.Condition != "BAD" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Condition must be EXCELLENT, GOOD, or BAD"})
		return
	}

	var reservation models.Reservation
	if err := db.Where("reservation_uid = ? AND username = ?", reservationUid, username).First(&reservation).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Reservation not found"})
		return
	}

	if request.Status == "EXPIRED" || request.Status == "RETURNED" {
		reservation.Status = request.Status
	} else {
		returnDate, err := time.Parse("2006-01-02", request.Date)
		if err == nil {
			if returnDate.After(reservation.TillDate) {
				reservation.Status = "EXPIRED"
			} else {
				reservation.Status = "RETURNED"
			}
		} else {
			reservation.Status = "RETURNED"
		}
	}

	if err := db.Save(&reservation).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.Data(http.StatusNoContent, "application/json", nil)
}

func seedTestData() {
	reservations := []models.Reservation{
		{
			Username:   "alice",
			BookUid:    "book1",
			LibraryUid: "lib1",
			Status:     "active",
			StartDate:  time.Now(),
			TillDate:   time.Now().AddDate(0, 0, 7),
		},
		{
			Username:   "bob",
			BookUid:    "book2",
			LibraryUid: "lib2",
			Status:     "completed",
			StartDate:  time.Now().AddDate(0, 0, -7),
			TillDate:   time.Now().AddDate(0, 0, -1),
		},
	}

	for _, res := range reservations {
		var existing models.Reservation
		if err := db.Where("username = ? AND book_uid = ?", res.Username, res.BookUid).First(&existing).Error; err != nil {
			db.Create(&res)
		}
	}
	log.Println("Reservation test data seeded")
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
		"details": "Host localhost:8070 is active",
	})
}

func getEnv(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
}
