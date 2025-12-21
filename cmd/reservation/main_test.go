package main

import (
	"RSOI_lab_3/pkg/models"
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

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
	db.AutoMigrate(&models.Reservation{})
	return db
}

func TestGetReservations(t *testing.T) {
	gin.SetMode(gin.TestMode)
	testDB := setupTestDB()
	db = testDB

	testReservation := models.Reservation{
		ReservationUid: "test-res-uid",
		Username:       "testuser",
		BookUid:        "test-book-uid",
		LibraryUid:     "test-lib-uid",
		Status:         "RENTED",
		BookCondition:  "EXCELLENT",
		StartDate:      time.Now(),
		TillDate:       time.Now().AddDate(0, 0, 7),
	}
	testDB.Create(&testReservation)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/api/v1/reservations", nil)
	c.Request.Header.Set("X-User-Name", "testuser")

	getReservations(c)

	assert.Equal(t, http.StatusOK, w.Code)
	var response []map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)
	assert.Equal(t, 1, len(response))
	assert.Equal(t, "test-res-uid", response[0]["reservationUid"])
}

func TestGetReservationsMissingHeader(t *testing.T) {
	gin.SetMode(gin.TestMode)
	testDB := setupTestDB()
	db = testDB

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/api/v1/reservations", nil)

	getReservations(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestGetActiveReservationsCount(t *testing.T) {
	gin.SetMode(gin.TestMode)
	testDB := setupTestDB()
	db = testDB

	testReservation1 := models.Reservation{
		ReservationUid: "test-res-uid-1",
		Username:       "testuser",
		BookUid:        "test-book-uid-1",
		LibraryUid:     "test-lib-uid",
		Status:         "RENTED",
		BookCondition:  "EXCELLENT",
		StartDate:      time.Now(),
		TillDate:       time.Now().AddDate(0, 0, 7),
	}
	testDB.Create(&testReservation1)

	testReservation2 := models.Reservation{
		ReservationUid: "test-res-uid-2",
		Username:       "testuser",
		BookUid:        "test-book-uid-2",
		LibraryUid:     "test-lib-uid",
		Status:         "RETURNED",
		BookCondition:  "EXCELLENT",
		StartDate:      time.Now().AddDate(0, 0, -10),
		TillDate:       time.Now().AddDate(0, 0, -3),
	}
	testDB.Create(&testReservation2)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/api/v1/reservations/active/count", nil)
	c.Request.Header.Set("X-User-Name", "testuser")

	getActiveReservationsCount(c)

	assert.Equal(t, http.StatusOK, w.Code)
	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)
	assert.Equal(t, float64(1), response["count"])
}

func TestCreateReservations(t *testing.T) {
	gin.SetMode(gin.TestMode)
	testDB := setupTestDB()
	db = testDB

	requestBody := map[string]interface{}{
		"bookUid":       "test-book-uid",
		"libraryUid":    "test-lib-uid",
		"tillDate":      time.Now().AddDate(0, 0, 7).Format("2006-01-02"),
		"bookCondition": "EXCELLENT",
	}
	jsonBody, _ := json.Marshal(requestBody)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/api/v1/reservations", bytes.NewBuffer(jsonBody))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Request.Header.Set("X-User-Name", "testuser")

	createReservations(c)

	assert.Equal(t, http.StatusOK, w.Code)
	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)
	assert.NotNil(t, response["reservationUid"])
	assert.Equal(t, "RENTED", response["status"])

	var reservation models.Reservation
	testDB.Where("username = ?", "testuser").First(&reservation)
	assert.Equal(t, "RENTED", reservation.Status)
}

func TestCreateReservationsMissingHeader(t *testing.T) {
	gin.SetMode(gin.TestMode)
	testDB := setupTestDB()
	db = testDB

	requestBody := map[string]interface{}{
		"bookUid":    "test-book-uid",
		"libraryUid": "test-lib-uid",
		"tillDate":   time.Now().AddDate(0, 0, 7).Format("2006-01-02"),
	}
	jsonBody, _ := json.Marshal(requestBody)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/api/v1/reservations", bytes.NewBuffer(jsonBody))
	c.Request.Header.Set("Content-Type", "application/json")

	createReservations(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestReturnBook(t *testing.T) {
	gin.SetMode(gin.TestMode)
	testDB := setupTestDB()
	db = testDB

	testReservation := models.Reservation{
		ReservationUid: "test-res-uid",
		Username:       "testuser",
		BookUid:        "test-book-uid",
		LibraryUid:     "test-lib-uid",
		Status:         "RENTED",
		BookCondition:  "EXCELLENT",
		StartDate:      time.Now(),
		TillDate:       time.Now().AddDate(0, 0, 7),
	}
	testDB.Create(&testReservation)

	requestBody := map[string]interface{}{
		"condition": "GOOD",
		"date":      time.Now().Format("2006-01-02"),
		"status":    "RETURNED",
	}
	jsonBody, _ := json.Marshal(requestBody)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/api/v1/reservations/test-res-uid/return", bytes.NewBuffer(jsonBody))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Request.Header.Set("X-User-Name", "testuser")
	c.Params = gin.Params{gin.Param{Key: "reservationUid", Value: "test-res-uid"}}

	returnBook(c)

	if w.Code != http.StatusNoContent {
		t.Logf("Response body: %s", w.Body.String())
	}
	assert.Equal(t, http.StatusNoContent, w.Code)

	var reservation models.Reservation
	testDB.Where("reservation_uid = ?", "test-res-uid").First(&reservation)
	assert.Equal(t, "RETURNED", reservation.Status)
}

func TestReturnBookExpired(t *testing.T) {
	gin.SetMode(gin.TestMode)
	testDB := setupTestDB()
	db = testDB

	testReservation := models.Reservation{
		ReservationUid: "test-res-uid",
		Username:       "testuser",
		BookUid:        "test-book-uid",
		LibraryUid:     "test-lib-uid",
		Status:         "RENTED",
		BookCondition:  "EXCELLENT",
		StartDate:      time.Now().AddDate(0, 0, -10),
		TillDate:       time.Now().AddDate(0, 0, -3),
	}
	testDB.Create(&testReservation)

	requestBody := map[string]interface{}{
		"condition": "GOOD",
		"date":      time.Now().Format("2006-01-02"),
		"status":    "EXPIRED",
	}
	jsonBody, _ := json.Marshal(requestBody)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/api/v1/reservations/test-res-uid/return", bytes.NewBuffer(jsonBody))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Request.Header.Set("X-User-Name", "testuser")
	c.Params = gin.Params{gin.Param{Key: "reservationUid", Value: "test-res-uid"}}

	returnBook(c)

	assert.Equal(t, http.StatusNoContent, w.Code)

	var reservation models.Reservation
	testDB.Where("reservation_uid = ?", "test-res-uid").First(&reservation)
	assert.Equal(t, "EXPIRED", reservation.Status)
}
