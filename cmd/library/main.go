package main

import (
	"RSOI_lab_2/pkg/models"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

var db *gorm.DB

func main() {
	log.Println("Starting library service...")

	host := getEnv("DB_HOST", "postgres")
	port := getEnv("DB_PORT", "5432")
	user := getEnv("DB_USER", "program")
	password := getEnv("DB_PASSWORD", "test")
	dbname := getEnv("DB_NAME", "libraries")

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

	err = db.AutoMigrate(&models.Library{}, &models.Book{}, &models.LibraryBook{})
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
	server.GET("/api/v1/libraries", getLibraries)
	server.GET("/api/v1/libraries/:libraryUid", getLibrary)
	server.GET("/api/v1/libraries/:libraryUid/books", getLibraryBooks)
	server.GET("/api/v1/libraries/:libraryUid/books/:bookUid", getLibraryBook)
	server.POST("/api/v1/libraries/:libraryUid/books/:bookUid/decrease", decreaseBookCount)
	server.POST("/api/v1/libraries/:libraryUid/books/:bookUid/increase", increaseBookCount)
	server.GET("/manage/health", healthCheck)

	log.Println("Library service starting on :8060")
	if err := server.Run(":8060"); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}

func getLibraries(c *gin.Context) {
	city := c.Query("city")
	pagestr := c.DefaultQuery("page", "1")
	sizestr := c.DefaultQuery("size", "10")

	page, err := strconv.Atoi(pagestr)
	if err != nil || page < 1 {
		page = 1
	}

	size, err := strconv.Atoi(sizestr)
	if err != nil || size < 1 || size > 100 {
		size = 10
	}

	if city == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "city is required"})
		return
	}
	var libraries []models.Library
	query := db.Where("city = ?", city)
	var totalelem int64
	query.Model(&libraries).Count(&totalelem)

	offset := (page - 1) * size
	err = query.Offset(offset).Limit(size).Find(&libraries).Error
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	items := make([]gin.H, len(libraries))
	for i, lib := range libraries {
		items[i] = gin.H{
			"libraryUid": lib.LibraryUid,
			"name":       lib.Name,
			"address":    lib.Address,
			"city":       lib.City,
		}
	}
	c.JSON(http.StatusOK, gin.H{
		"page":          page,
		"pageSize":      size,
		"totalElements": totalelem,
		"items":         items,
	})
}

func getLibraryBooks(c *gin.Context) {
	libraryUid := c.Param("libraryUid")
	pageStr := c.DefaultQuery("page", "1")
	sizeStr := c.DefaultQuery("size", "10")
	showAll := c.DefaultQuery("showall", "false")

	page, err := strconv.Atoi(pageStr)
	if err != nil || page < 1 {
		page = 1
	}

	size, err := strconv.Atoi(sizeStr)
	if err != nil || size < 1 || size > 100 {
		size = 10
	}

	showall := showAll == "true"

	var library models.Library
	err = db.Where("library_uid = ?", libraryUid).First(&library).Error
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "library not found"})
		return
	}
	var libraryBooks []models.LibraryBook
	query := db.Where("library_id = ?", library.ID).Preload("Book")

	if !showall {
		query = query.Where("available_count > 0")
	}

	var totalelem int64
	query.Model(&models.LibraryBook{}).Count(&totalelem)

	offset := (page - 1) * size
	err = query.Offset(offset).Limit(size).Find(&libraryBooks).Error
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	items := make([]gin.H, len(libraryBooks))
	for i, lb := range libraryBooks {
		items[i] = gin.H{
			"bookUid":        lb.Book.BookUid,
			"name":           lb.Book.Name,
			"author":         lb.Book.Author,
			"genre":          lb.Book.Genre,
			"condition":      lb.Book.Condition,
			"availableCount": lb.AvailableCount,
		}
	}
	c.JSON(http.StatusOK, gin.H{
		"page":          page,
		"pageSize":      size,
		"totalElements": totalelem,
		"items":         items,
	})
}

func getLibrary(c *gin.Context) {
	libraryUid := c.Param("libraryUid")

	var library models.Library
	if err := db.Where("library_uid = ?", libraryUid).First(&library).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Library not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"libraryUid": library.LibraryUid,
		"name":       library.Name,
		"address":    library.Address,
		"city":       library.City,
	})
}

func getLibraryBook(c *gin.Context) {
	libraryUid := c.Param("libraryUid")
	bookUid := c.Param("bookUid")

	var library models.Library
	if err := db.Where("library_uid = ?", libraryUid).First(&library).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Library not found"})
		return
	}

	var book models.Book
	if err := db.Where("book_uid = ?", bookUid).First(&book).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Book not found"})
		return
	}

	var libraryBook models.LibraryBook
	if err := db.Where("library_id = ? AND book_id = ?", library.ID, book.ID).
		First(&libraryBook).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Book not found in library"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"bookUid":        book.BookUid,
		"name":           book.Name,
		"author":         book.Author,
		"genre":          book.Genre,
		"condition":      book.Condition,
		"availableCount": libraryBook.AvailableCount,
	})
}

func decreaseBookCount(c *gin.Context) {
	libraryUid := c.Param("libraryUid")
	bookUid := c.Param("bookUid")

	var library models.Library
	if err := db.Where("library_uid = ?", libraryUid).First(&library).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Library not found"})
		return
	}

	var book models.Book
	if err := db.Where("book_uid = ?", bookUid).First(&book).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Book not found"})
		return
	}

	var libraryBook models.LibraryBook
	if err := db.Where("library_id = ? AND book_id = ?", library.ID, book.ID).
		First(&libraryBook).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Book not found in library"})
		return
	}

	if libraryBook.AvailableCount <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Book not available"})
		return
	}

	libraryBook.AvailableCount--
	if err := db.Save(&libraryBook).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update book count"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"bookUid":        book.BookUid,
		"availableCount": libraryBook.AvailableCount,
	})
}

func increaseBookCount(c *gin.Context) {
	libraryUid := c.Param("libraryUid")
	bookUid := c.Param("bookUid")

	var library models.Library
	if err := db.Where("library_uid = ?", libraryUid).First(&library).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Library not found"})
		return
	}

	var book models.Book
	if err := db.Where("book_uid = ?", bookUid).First(&book).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Book not found"})
		return
	}

	var libraryBook models.LibraryBook
	if err := db.Where("library_id = ? AND book_id = ?", library.ID, book.ID).
		First(&libraryBook).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Book not found in library"})
		return
	}

	libraryBook.AvailableCount++
	if err := db.Save(&libraryBook).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update book count"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"bookUid":        book.BookUid,
		"availableCount": libraryBook.AvailableCount,
	})
}

func seedTestData() {
	testLibraryUid := "83575e12-7ce0-48ee-9931-51919ff3c9ee"
	testBookUid := "f7cdc58f-2caf-4b15-9727-f89dcc629b27"

	var testLib models.Library
	if err := db.Where("library_uid = ?", testLibraryUid).First(&testLib).Error; err != nil {
		testLib = models.Library{
			LibraryUid: testLibraryUid,
			Name:       "Библиотека имени 7 Непьющих",
			Address:    "2-я Бауманская ул., д.5, стр.1",
			City:       "Москва",
		}
		if err := db.Create(&testLib).Error; err != nil {
			log.Printf("Failed to create test library: %v", err)
		} else {
			log.Printf("Created test library: %s", testLib.Name)
		}
	}
	if testLib.Name != "Библиотека имени 7 Непьющих" || testLib.City != "Москва" {
		testLib.Name = "Библиотека имени 7 Непьющих"
		testLib.Address = "2-я Бауманская ул., д.5, стр.1"
		testLib.City = "Москва"
		db.Save(&testLib)
	}

	var testBook models.Book
	if err := db.Where("book_uid = ?", testBookUid).First(&testBook).Error; err != nil {
		testBook = models.Book{
			BookUid:   testBookUid,
			Name:      "Краткий курс C++ в 7 томах",
			Author:    "Бьерн Страуструп",
			Genre:     "Научная фантастика",
			Condition: "EXCELLENT",
		}
		if err := db.Create(&testBook).Error; err != nil {
			log.Printf("Failed to create test book: %v", err)
		} else {
			log.Printf("Created test book: %s", testBook.Name)
		}
	} else {
		testBook.Name = "Краткий курс C++ в 7 томах"
		testBook.Author = "Бьерн Страуструп"
		testBook.Genre = "Научная фантастика"
		testBook.Condition = "EXCELLENT"
		db.Save(&testBook)
	}

	var libraryBook models.LibraryBook
	if err := db.Where("library_id = ? AND book_id = ?", testLib.ID, testBook.ID).
		First(&libraryBook).Error; err != nil {
		libraryBook = models.LibraryBook{
			LibraryID:      testLib.ID,
			BookID:         testBook.ID,
			AvailableCount: 1,
		}
		if err := db.Create(&libraryBook).Error; err != nil {
			log.Printf("Failed to link book to library: %v", err)
		} else {
			log.Printf("Linked book %s to library %s with available_count: %d",
				testBook.Name, testLib.Name, libraryBook.AvailableCount)
		}
	} else {
		libraryBook.AvailableCount = 1
		db.Save(&libraryBook)
	}

	libraries := []models.Library{
		{Name: "Central Library", Address: "123 Main St", City: "Moscow"},
		{Name: "North Library", Address: "456 North Ave", City: "Moscow"},
		{Name: "South Library", Address: "789 South St", City: "St Petersburg"},
	}

	for _, lib := range libraries {
		var existing models.Library
		if err := db.Where("name = ?", lib.Name).First(&existing).Error; err != nil {
			lib.LibraryUid = uuid.New().String()
			if err := db.Create(&lib).Error; err != nil {
				log.Printf("Failed to create library %s: %v", lib.Name, err)
			}
		}
	}
	log.Println("Library test data seeded")
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
		"details": "Host localhost:8060 is active",
	})
}

func getEnv(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
}
