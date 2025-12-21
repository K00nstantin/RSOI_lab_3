package main

import (
	"RSOI_lab_3/pkg/models"
	"bytes"
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
	db.AutoMigrate(&models.Rating{})
	return db
}

func TestGetRating(t *testing.T) {
	gin.SetMode(gin.TestMode)
	testDB := setupTestDB()
	db = testDB

	testRating := models.Rating{
		Username: "testuser",
		Stars:    75,
	}
	testDB.Create(&testRating)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/api/v1/rating", nil)
	c.Request.Header.Set("X-User-Name", "testuser")

	getRating(c)

	assert.Equal(t, http.StatusOK, w.Code)
	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)
	assert.Equal(t, float64(75), response["stars"])
}

func TestGetRatingMissingHeader(t *testing.T) {
	gin.SetMode(gin.TestMode)
	testDB := setupTestDB()
	db = testDB

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/api/v1/rating", nil)

	getRating(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestGetRatingNewUser(t *testing.T) {
	gin.SetMode(gin.TestMode)
	testDB := setupTestDB()
	db = testDB

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/api/v1/rating", nil)
	c.Request.Header.Set("X-User-Name", "newuser")

	getRating(c)

	assert.Equal(t, http.StatusOK, w.Code)
	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)
	assert.Equal(t, float64(1), response["stars"])

	var rating models.Rating
	testDB.Where("username = ?", "newuser").First(&rating)
	assert.Equal(t, 1, rating.Stars)
}

func TestUpdateRating(t *testing.T) {
	gin.SetMode(gin.TestMode)
	testDB := setupTestDB()
	db = testDB

	testRating := models.Rating{
		Username: "testuser",
		Stars:    50,
	}
	testDB.Create(&testRating)

	requestBody := map[string]interface{}{
		"username": "testuser",
		"stars":    80,
	}
	jsonBody, _ := json.Marshal(requestBody)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("PUT", "/api/v1/rating", bytes.NewBuffer(jsonBody))
	c.Request.Header.Set("Content-Type", "application/json")

	updateRating(c)

	assert.Equal(t, http.StatusOK, w.Code)
	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)
	assert.Equal(t, float64(80), response["stars"])

	var updatedRating models.Rating
	testDB.Where("username = ?", "testuser").First(&updatedRating)
	assert.Equal(t, 80, updatedRating.Stars)
}
