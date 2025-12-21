package database

import (
	"RSOI_lab_2/pkg/models"
	"fmt"
	"log"
	"os"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func InitRatingDB() *gorm.DB {
	host := getEnv("DB_HOST", "postgres") // ИЗМЕНИТЕ: postgres вместо localhost
	port := getEnv("DB_PORT", "5432")
	user := getEnv("DB_USER", "program")
	password := getEnv("DB_PASSWORD", "test")
	dbname := getEnv("DB_NAME", "ratings")

	dsn := fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%s sslmode=disable TimeZone=UTC",
		host, user, password, dbname, port)

	log.Printf("Connecting to rating database: host=%s, port=%s", host, port)
	return initDB(dsn, &models.Rating{})
}

func InitLibraryDB() *gorm.DB {
	host := getEnv("DB_HOST", "postgres") // ИЗМЕНИТЕ: postgres вместо localhost
	port := getEnv("DB_PORT", "5432")
	user := getEnv("DB_USER", "program")
	password := getEnv("DB_PASSWORD", "test")
	dbname := getEnv("DB_NAME", "library") // Исправьте на "library" если нужно

	dsn := fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%s sslmode=disable TimeZone=UTC",
		host, user, password, dbname, port)

	log.Printf("Connecting to library database: host=%s, port=%s", host, port)
	db := initDB(dsn, &models.Library{}, &models.Book{}, &models.LibraryBook{})

	return db
}

func InitReservationDB() *gorm.DB {
	host := getEnv("DB_HOST", "postgres") // ИЗМЕНИТЕ: postgres вместо localhost
	port := getEnv("DB_PORT", "5432")
	user := getEnv("DB_USER", "program")
	password := getEnv("DB_PASSWORD", "test")
	dbname := getEnv("DB_NAME", "reservation")

	dsn := fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%s sslmode=disable TimeZone=UTC",
		host, user, password, dbname, port)

	log.Printf("Connecting to reservation database: host=%s, port=%s", host, port)
	return initDB(dsn, &models.Reservation{})
}

func initDB(dsn string, models ...interface{}) *gorm.DB {
	log.Printf("Database DSN: %s", dsn)

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatal("Failed to connect to database:", err)
	}

	sqlDB, _ := db.DB()
	sqlDB.SetMaxOpenConns(25)
	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetConnMaxLifetime(5 * time.Minute)

	err = db.AutoMigrate(models...)
	if err != nil {
		log.Fatal("Database migration failed:", err)
	}

	log.Println("Database connection established successfully")
	return db
}

func getEnv(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
}
