package main

import (
	"RSOI_lab_3/pkg/models"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupTestDB() *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		panic("failed to connect test database")
	}
	db.AutoMigrate(&models.Library{}, &models.Book{}, &models.LibraryBook{})
	return db
}

func TestGetLibraries(t *testing.T) {
	gin.SetMode(gin.TestMode)
	testDB := setupTestDB()
	db = testDB

	testLib := models.Library{
		LibraryUid: "test-lib-uid",
		Name:       "Test Library",
		City:       "Moscow",
		Address:    "Test Address",
	}
	testDB.Create(&testLib)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/api/v1/libraries?city=Moscow&page=1&size=10", nil)

	getLibraries(c)

	assert.Equal(t, http.StatusOK, w.Code)
	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)
	assert.NotNil(t, response["items"])
	items := response["items"].([]interface{})
	assert.Equal(t, 1, len(items))
}

func TestGetLibrary(t *testing.T) {
	gin.SetMode(gin.TestMode)
	testDB := setupTestDB()
	db = testDB

	testLib := models.Library{
		LibraryUid: "test-lib-uid",
		Name:       "Test Library",
		City:       "Moscow",
		Address:    "Test Address",
	}
	testDB.Create(&testLib)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/api/v1/libraries/test-lib-uid", nil)
	c.Params = gin.Params{gin.Param{Key: "libraryUid", Value: "test-lib-uid"}}

	getLibrary(c)

	assert.Equal(t, http.StatusOK, w.Code)
	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)
	assert.Equal(t, "test-lib-uid", response["libraryUid"])
	assert.Equal(t, "Test Library", response["name"])
}

func TestGetLibraryBooks(t *testing.T) {
	gin.SetMode(gin.TestMode)
	testDB := setupTestDB()
	db = testDB

	testLib := models.Library{
		LibraryUid: "test-lib-uid",
		Name:       "Test Library",
		City:       "Moscow",
		Address:    "Test Address",
	}
	testDB.Create(&testLib)

	testBook := models.Book{
		BookUid:   "test-book-uid",
		Name:      "Test Book",
		Author:    "Test Author",
		Genre:     "Fiction",
		Condition: "EXCELLENT",
	}
	testDB.Create(&testBook)

	libraryBook := models.LibraryBook{
		LibraryID:      testLib.ID,
		BookID:         testBook.ID,
		AvailableCount: 5,
	}
	testDB.Create(&libraryBook)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/api/v1/libraries/test-lib-uid/books?page=1&size=10", nil)
	c.Params = gin.Params{gin.Param{Key: "libraryUid", Value: "test-lib-uid"}}

	getLibraryBooks(c)

	assert.Equal(t, http.StatusOK, w.Code)
	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)
	assert.NotNil(t, response["items"])
	items := response["items"].([]interface{})
	assert.Equal(t, 1, len(items))
}

func TestDecreaseBookCount(t *testing.T) {
	gin.SetMode(gin.TestMode)
	testDB := setupTestDB()
	db = testDB

	testLib := models.Library{
		LibraryUid: "test-lib-uid",
		Name:       "Test Library",
		City:       "Moscow",
		Address:    "Test Address",
	}
	testDB.Create(&testLib)

	testBook := models.Book{
		BookUid:   "test-book-uid",
		Name:      "Test Book",
		Author:    "Test Author",
		Genre:     "Fiction",
		Condition: "EXCELLENT",
	}
	testDB.Create(&testBook)

	libraryBook := models.LibraryBook{
		LibraryID:      testLib.ID,
		BookID:         testBook.ID,
		AvailableCount: 5,
	}
	testDB.Create(&libraryBook)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/api/v1/libraries/test-lib-uid/books/test-book-uid/decrease", nil)
	c.Params = gin.Params{
		gin.Param{Key: "libraryUid", Value: "test-lib-uid"},
		gin.Param{Key: "bookUid", Value: "test-book-uid"},
	}

	decreaseBookCount(c)

	assert.Equal(t, http.StatusOK, w.Code)
	var updatedLibraryBook models.LibraryBook
	testDB.Where("library_id = ? AND book_id = ?", testLib.ID, testBook.ID).First(&updatedLibraryBook)
	assert.Equal(t, 4, updatedLibraryBook.AvailableCount)
}

func TestIncreaseBookCount(t *testing.T) {
	gin.SetMode(gin.TestMode)
	testDB := setupTestDB()
	db = testDB

	testLib := models.Library{
		LibraryUid: "test-lib-uid",
		Name:       "Test Library",
		City:       "Moscow",
		Address:    "Test Address",
	}
	testDB.Create(&testLib)

	testBook := models.Book{
		BookUid:   "test-book-uid",
		Name:      "Test Book",
		Author:    "Test Author",
		Genre:     "Fiction",
		Condition: "EXCELLENT",
	}
	testDB.Create(&testBook)

	libraryBook := models.LibraryBook{
		LibraryID:      testLib.ID,
		BookID:         testBook.ID,
		AvailableCount: 5,
	}
	testDB.Create(&libraryBook)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/api/v1/libraries/test-lib-uid/books/test-book-uid/increase", nil)
	c.Params = gin.Params{
		gin.Param{Key: "libraryUid", Value: "test-lib-uid"},
		gin.Param{Key: "bookUid", Value: "test-book-uid"},
	}

	increaseBookCount(c)

	assert.Equal(t, http.StatusOK, w.Code)
	var updatedLibraryBook models.LibraryBook
	testDB.Where("library_id = ? AND book_id = ?", testLib.ID, testBook.ID).First(&updatedLibraryBook)
	assert.Equal(t, 6, updatedLibraryBook.AvailableCount)
}
