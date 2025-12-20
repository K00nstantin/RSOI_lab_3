package main

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestIsConditionWorse(t *testing.T) {
	tests := []struct {
		name              string
		originalCondition string
		returnedCondition string
		expected          bool
	}{
		{
			name:              "EXCELLENT to GOOD is worse",
			originalCondition: "EXCELLENT",
			returnedCondition: "GOOD",
			expected:          true,
		},
		{
			name:              "EXCELLENT to BAD is worse",
			originalCondition: "EXCELLENT",
			returnedCondition: "BAD",
			expected:          true,
		},
		{
			name:              "GOOD to BAD is worse",
			originalCondition: "GOOD",
			returnedCondition: "BAD",
			expected:          true,
		},
		{
			name:              "EXCELLENT to EXCELLENT is not worse",
			originalCondition: "EXCELLENT",
			returnedCondition: "EXCELLENT",
			expected:          false,
		},
		{
			name:              "GOOD to EXCELLENT is not worse",
			originalCondition: "GOOD",
			returnedCondition: "EXCELLENT",
			expected:          false,
		},
		{
			name:              "BAD to GOOD is not worse",
			originalCondition: "BAD",
			returnedCondition: "GOOD",
			expected:          false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isConditionWorse(tt.originalCondition, tt.returnedCondition)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetLibrariesHandler(t *testing.T) {
	gin.SetMode(gin.TestMode)

	libraryServiceURL = "http://invalid-url"
	httpClient = &http.Client{}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/api/v1/libraries?city=Moscow&page=1&size=10", nil)

	getLibrariesHandler(c)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestGetRatingHandler(t *testing.T) {
	gin.SetMode(gin.TestMode)

	ratingServiceURL = "http://invalid-url"
	httpClient = &http.Client{}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/api/v1/rating", nil)
	c.Request.Header.Set("X-User-Name", "testuser")

	getRatingHandler(c)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

