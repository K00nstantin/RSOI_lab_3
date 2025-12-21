package main

import (
	"RSOI_lab_3/pkg/circuitbreaker"
	"RSOI_lab_3/pkg/queue"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

var (
	ratingServiceURL, libraryServiceURL, reservationServiceURL string
	httpClient                                                 *http.Client
	libraryCB, ratingCB, reservationCB                         *circuitbreaker.CircuitBreaker
	retryQueue                                                 *queue.Queue
)

const (
	maxFailures = 3
	timeout     = 30 * time.Second
	retryDelay  = 10 * time.Second
	maxRetries  = 5
)

func main() {
	ratingServiceURL = getEnv("RATING_SERVICE_URL", "http://localhost:8050")
	libraryServiceURL = getEnv("LIBRARY_SERVICE_URL", "http://localhost:8060")
	reservationServiceURL = getEnv("RESERVATION_SERVICE_URL", "http://localhost:8070")

	httpClient = &http.Client{Timeout: 10 * time.Second}
	libraryCB = circuitbreaker.NewCircuitBreaker(maxFailures, timeout)
	ratingCB = circuitbreaker.NewCircuitBreaker(maxFailures, timeout)
	reservationCB = circuitbreaker.NewCircuitBreaker(maxFailures, timeout)
	retryQueue = queue.NewQueue()

	go processRetryQueue()

	r := gin.Default()
	r.GET("/api/v1/libraries", getLibrariesHandler)
	r.GET("/api/v1/libraries/:libraryUid/books", getLibraryBooksHandler)
	r.GET("/api/v1/reservations", getReservationsHandler)
	r.POST("/api/v1/reservations", createReservationHandler)
	r.POST("/api/v1/reservations/:reservationUid/return", returnBookHandler)
	r.GET("/api/v1/rating", getRatingHandler)
	r.GET("/manage/health", healthCheck)

	log.Println("Gateway service starting on port 8080")
	r.Run(":8080")
}

func processRetryQueue() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for range ticker.C {
		for req := retryQueue.Dequeue(); req != nil; req = retryQueue.Dequeue() {
			log.Printf("Retrying request %s (attempt %d/%d)", req.ID, req.RetryCount+1, req.MaxRetries)
			if !executeRetryRequest(req) {
				req.RetryCount++
				if req.RetryCount < req.MaxRetries {
					req.RetryAt = time.Now().Add(retryDelay)
					retryQueue.Enqueue(req)
				}
			}
		}
	}
}

func executeRetryRequest(req *queue.RetryRequest) bool {
	httpReq, err := http.NewRequest(req.Method, req.URL, bytes.NewBuffer(req.Body))
	if err != nil {
		return false
	}
	for k, v := range req.Headers {
		httpReq.Header.Set(k, v)
	}
	resp, err := httpClient.Do(httpReq)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode >= 200 && resp.StatusCode < 300
}

func executeWithCB(cb *circuitbreaker.CircuitBreaker, c *gin.Context, method, url string, body []byte, headers map[string]string, fallback func()) (*http.Response, error) {
	var resp *http.Response
	var fallbackCalled bool

	err := cb.Execute(
		func() error {
			req, err := http.NewRequest(method, url, bytes.NewBuffer(body))
			if err != nil {
				return err
			}
			for k, v := range headers {
				req.Header.Set(k, v)
			}
			resp, err = httpClient.Do(req)
			if err != nil {
				return err
			}
			if resp.StatusCode != http.StatusOK {
				resp.Body.Close()
				return fmt.Errorf("status %d", resp.StatusCode)
			}
			return nil
		},
		func() error {
			fallbackCalled = true
			fallback()
			return nil
		},
	)

	if fallbackCalled {
		return nil, nil
	}
	return resp, err
}

func getLibrariesHandler(c *gin.Context) {
	params := c.Request.URL.Query().Encode()
	url := libraryServiceURL + "/api/v1/libraries"
	if params != "" {
		url += "?" + params
	}
	resp, _ := executeWithCB(libraryCB, c, "GET", url, nil, nil, func() {
		c.JSON(200, gin.H{"page": 1, "pageSize": 10, "totalElements": 0, "items": []interface{}{}})
	})
	if resp == nil {
		return
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	c.Data(resp.StatusCode, "application/json", body)
}

func getLibraryBooksHandler(c *gin.Context) {
	libraryUid := c.Param("libraryUid")
	queryparams := c.Request.URL.Query().Encode()
	url := fmt.Sprintf("%s/api/v1/libraries/%s/books", libraryServiceURL, libraryUid)
	if queryparams != "" {
		url += "?" + queryparams
	}
	resp, _ := executeWithCB(libraryCB, c, "GET", url, nil, nil, func() {
		c.JSON(200, gin.H{"page": 1, "pageSize": 10, "totalElements": 0, "items": []interface{}{}})
	})
	if resp == nil {
		return
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	c.Data(resp.StatusCode, "application/json", body)
}

func getReservationsHandler(c *gin.Context) {
	username := c.GetHeader("X-User-Name")
	if username == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "X-User-Name header is required"})
		return
	}
	url := reservationServiceURL + "/api/v1/reservations"
	resp, _ := executeWithCB(reservationCB, c, "GET", url, nil,
		map[string]string{"X-User-Name": username}, func() {
			c.JSON(200, []interface{}{})
		})

	if resp == nil {
		return
	}
	defer resp.Body.Close()

	var reservations []map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&reservations)
	enrichedReservations := make([]map[string]interface{}, len(reservations))
	for i, res := range reservations {
		bookUid, _ := res["bookUid"].(string)
		libraryUid, _ := res["libraryUid"].(string)
		bookInfo := getBookInfoWithFallback(libraryUid, bookUid)
		libraryInfo := getLibraryInfoWithFallback(libraryUid)
		enrichedReservations[i] = map[string]interface{}{
			"reservationUid": res["reservationUid"],
			"status":         res["status"],
			"startDate":      res["startDate"],
			"tillDate":       res["tillDate"],
			"book":           bookInfo,
			"library":        libraryInfo,
		}
	}
	c.JSON(http.StatusOK, enrichedReservations)
}

func createReservationHandler(c *gin.Context) {
	username := c.GetHeader("X-User-Name")
	if username == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "X-User-Name header must be contained"})
		return
	}
	var request struct {
		BookUid    string `json:"bookUid" binding:"required"`
		LibraryUid string `json:"libraryUid" binding:"required"`
		TillDate   string `json:"tillDate" binding:"required"`
	}
	err := c.ShouldBindJSON(&request)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"message": "validation error",
			"errors": map[string]string{
				"field": "request",
				"error": err.Error(),
			},
		})
		return
	}
	bookinfo := getBookInfoWithFallback(request.LibraryUid, request.BookUid)
	if bookinfo == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to find the book in the library"})
		return
	}
	availableCount, ok := bookinfo["availableCount"].(float64)
	if !ok || availableCount <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "book not available"})
		return
	}

	activeReservationsCount := getActiveReservationsCountWithFallback(username)
	rating := getUserRatingWithFallback(username)
	stars, ok := rating["stars"].(float64)
	if !ok {
		stars = 0
	}

	if activeReservationsCount >= int(stars) {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "User has reached the maximum number of books allowed by rating",
		})
		return
	}

	bookCondition, ok := bookinfo["condition"].(string)
	if !ok {
		bookCondition = "EXCELLENT"
	}

	requestWithCondition := map[string]interface{}{
		"bookUid":       request.BookUid,
		"libraryUid":    request.LibraryUid,
		"tillDate":      request.TillDate,
		"bookCondition": bookCondition,
	}

	body, err := json.Marshal(requestWithCondition)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create request body"})
		return
	}
	url := reservationServiceURL + "/api/v1/reservations"
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(body))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create the request"})
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-User-Name", username)
	resp, err := httpClient.Do(req)
	if err != nil {
		queueRequestForRetry("POST", url, map[string]string{"Content-Type": "application/json", "X-User-Name": username}, body)
		c.JSON(200, gin.H{"message": "Reservation request queued for processing"})
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		rbody, _ := io.ReadAll(resp.Body)
		c.Data(resp.StatusCode, "application/json", rbody)
		return
	}
	var reservation map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&reservation)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to decode the response"})
		return
	}

	err = decreaseBookCount(request.LibraryUid, request.BookUid)
	if err != nil {
		if reservationUid, ok := reservation["reservationUid"].(string); ok {
			queueRequestForRetry("DELETE", fmt.Sprintf("%s/api/v1/reservations/%s/rollback", reservationServiceURL, reservationUid), map[string]string{"X-User-Name": username}, nil)
		}
		queueRequestForRetry("POST", url, map[string]string{"Content-Type": "application/json", "X-User-Name": username}, body)
		c.JSON(200, gin.H{"message": "Reservation request queued for processing"})
		return
	}

	libraryinfo := getLibraryInfoWithFallback(request.LibraryUid)
	rating = getUserRatingWithFallback(username)
	response := map[string]interface{}{
		"reservationUid": reservation["reservationUid"],
		"status":         reservation["status"],
		"startDate":      reservation["startDate"],
		"tillDate":       reservation["tillDate"],
		"book": map[string]interface{}{
			"bookUid": bookinfo["bookUid"],
			"name":    bookinfo["name"],
			"author":  bookinfo["author"],
			"genre":   bookinfo["genre"],
		},
		"library": libraryinfo,
		"rating":  rating,
	}
	c.JSON(http.StatusOK, response)
}

func returnBookHandler(c *gin.Context) {
	username := c.GetHeader("X-User-Name")
	if username == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "X-User-Name header is required"})
		return
	}
	reservationUid := c.Param("reservationUid")
	var request struct {
		Condition string `json:"condition" binding:"required"`
		Date      string `json:"date" binding:"required"`
	}
	err := c.ShouldBindJSON(&request)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"message": "validation error",
			"errors": map[string]string{
				"field": "request",
				"error": err.Error(),
			},
		})
		return
	}

	if request.Condition != "EXCELLENT" && request.Condition != "GOOD" && request.Condition != "BAD" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Condition must be EXCELLENT, GOOD, or BAD"})
		return
	}

	reservation, err := getReservationInfoWithFallback(reservationUid, username)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Reservation not found"})
		return
	}

	returnDate, err := time.Parse("2006-01-02", request.Date)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid date format. Use YYYY-MM-DD"})
		return
	}

	tillDate, err := time.Parse("2006-01-02", reservation["tillDate"].(string))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to parse reservation date"})
		return
	}

	status := "RETURNED"
	if returnDate.After(tillDate) {
		status = "EXPIRED"
	}

	reqbody, err := json.Marshal(map[string]interface{}{
		"condition": request.Condition,
		"date":      request.Date,
		"status":    status,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create request body"})
		return
	}

	url := fmt.Sprintf("%s/api/v1/reservations/%s/return", reservationServiceURL, reservationUid)
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(reqbody))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create the request"})
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-User-Name", username)
	resp, err := httpClient.Do(req)
	if err != nil {
		queueRequestForRetry("POST", url, map[string]string{"Content-Type": "application/json", "X-User-Name": username}, reqbody)
		c.Status(204)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(resp.Body)
		c.Data(resp.StatusCode, "application/json", body)
		return
	}

	libraryUid := reservation["libraryUid"].(string)
	bookUid := reservation["bookUid"].(string)
	err = increaseBookCount(libraryUid, bookUid)
	if err != nil {
		queueRequestForRetry("POST", fmt.Sprintf("%s/api/v1/reservations/%s/rollback-return", reservationServiceURL, reservationUid), map[string]string{"Content-Type": "application/json", "X-User-Name": username}, nil)
		queueRequestForRetry("POST", url, map[string]string{"Content-Type": "application/json", "X-User-Name": username}, reqbody)
		c.Status(204)
		return
	}

	bookConditionAtRental, ok := reservation["bookCondition"].(string)
	if !ok {
		bookConditionAtRental = "EXCELLENT"
	}

	isLate := returnDate.After(tillDate)
	isConditionWorse := isConditionWorse(bookConditionAtRental, request.Condition)
	isOnTimeAndGoodCondition := !isLate && !isConditionWorse

	var ratingDelta int
	if isOnTimeAndGoodCondition {
		ratingDelta = 1
	} else {
		if isLate {
			ratingDelta -= 10
		}
		if isConditionWorse {
			ratingDelta -= 10
		}
	}

	if ratingDelta != 0 {
		err = adjustUserRating(username, ratingDelta)
		if err != nil {
			log.Printf("Failed to update user rating: %v", err)
		}
	}

	c.Status(http.StatusNoContent)
}

func getRatingHandler(c *gin.Context) {
	username := c.GetHeader("X-User-Name")
	if username == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "X-User-Name header is required"})
		return
	}

	url := ratingServiceURL + "/api/v1/rating"
	resp, _ := executeWithCB(ratingCB, c, "GET", url, nil,
		map[string]string{"X-User-Name": username}, func() {
			c.JSON(200, gin.H{"stars": 0})
		})

	if resp == nil {
		return
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	c.Data(resp.StatusCode, "application/json", body)
}

func healthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":  "UP",
		"details": "Host localhost:8080 is active",
	})
}

func getEnv(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
}

func getBookInfoWithFallback(libraryUid, bookUid string) map[string]interface{} {
	var result map[string]interface{}
	libraryCB.Execute(
		func() error {
			req, _ := http.NewRequest("GET", fmt.Sprintf("%s/api/v1/libraries/%s/books/%s", libraryServiceURL, libraryUid, bookUid), nil)
			resp, err := httpClient.Do(req)
			if err != nil {
				return err
			}
			defer resp.Body.Close()
			if resp.StatusCode != 200 {
				return fmt.Errorf("status %d", resp.StatusCode)
			}
			return json.NewDecoder(resp.Body).Decode(&result)
		},
		func() error {
			result = map[string]interface{}{"bookUid": bookUid, "name": "", "author": "", "genre": "", "condition": "EXCELLENT", "availableCount": float64(0)}
			return nil
		},
	)
	return result
}

func getLibraryInfoWithFallback(libraryUid string) map[string]interface{} {
	var result map[string]interface{}
	libraryCB.Execute(
		func() error {
			req, _ := http.NewRequest("GET", fmt.Sprintf("%s/api/v1/libraries/%s", libraryServiceURL, libraryUid), nil)
			resp, err := httpClient.Do(req)
			if err != nil {
				return err
			}
			defer resp.Body.Close()
			if resp.StatusCode != 200 {
				return fmt.Errorf("status %d", resp.StatusCode)
			}
			return json.NewDecoder(resp.Body).Decode(&result)
		},
		func() error {
			result = map[string]interface{}{"libraryUid": libraryUid, "name": "", "address": "", "city": ""}
			return nil
		},
	)
	return result
}

func getUserRatingWithFallback(username string) map[string]interface{} {
	var result map[string]interface{}
	ratingCB.Execute(
		func() error {
			req, _ := http.NewRequest("GET", ratingServiceURL+"/api/v1/rating", nil)
			req.Header.Set("X-User-Name", username)
			resp, err := httpClient.Do(req)
			if err != nil {
				return err
			}
			defer resp.Body.Close()
			if resp.StatusCode != 200 {
				return fmt.Errorf("status %d", resp.StatusCode)
			}
			return json.NewDecoder(resp.Body).Decode(&result)
		},
		func() error {
			result = map[string]interface{}{"stars": 0}
			return nil
		},
	)
	return result
}

func getActiveReservationsCountWithFallback(username string) int {
	var count int
	reservationCB.Execute(
		func() error {
			req, _ := http.NewRequest("GET", reservationServiceURL+"/api/v1/reservations/active/count", nil)
			req.Header.Set("X-User-Name", username)
			resp, err := httpClient.Do(req)
			if err != nil {
				return err
			}
			defer resp.Body.Close()
			if resp.StatusCode != 200 {
				return fmt.Errorf("status %d", resp.StatusCode)
			}
			var result map[string]interface{}
			json.NewDecoder(resp.Body).Decode(&result)
			count = int(result["count"].(float64))
			return nil
		},
		func() error {
			count = 0
			return nil
		},
	)
	return count
}

func getReservationInfoWithFallback(reservationUid, username string) (map[string]interface{}, error) {
	var result map[string]interface{}
	var err error
	reservationCB.Execute(
		func() error {
			req, _ := http.NewRequest("GET", reservationServiceURL+"/api/v1/reservations", nil)
			req.Header.Set("X-User-Name", username)
			resp, err := httpClient.Do(req)
			if err != nil {
				return err
			}
			defer resp.Body.Close()
			if resp.StatusCode != 200 {
				return fmt.Errorf("status %d", resp.StatusCode)
			}
			var reservations []map[string]interface{}
			json.NewDecoder(resp.Body).Decode(&reservations)
			for _, r := range reservations {
				if r["reservationUid"] == reservationUid {
					result = r
					return nil
				}
			}
			return fmt.Errorf("not found")
		},
		func() error {
			err = fmt.Errorf("service unavailable")
			return nil
		},
	)
	return result, err
}

func decreaseBookCount(libraryUid, bookUid string) error {
	url := fmt.Sprintf("%s/api/v1/libraries/%s/books/%s/decrease", libraryServiceURL, libraryUid, bookUid)
	req, err := http.NewRequest("POST", url, nil)
	if err != nil {
		return err
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to decrease book count: status %d", resp.StatusCode)
	}

	return nil
}

func increaseBookCount(libraryUid, bookUid string) error {
	url := fmt.Sprintf("%s/api/v1/libraries/%s/books/%s/increase", libraryServiceURL, libraryUid, bookUid)
	req, err := http.NewRequest("POST", url, nil)
	if err != nil {
		return err
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to increase book count: status %d", resp.StatusCode)
	}

	return nil
}

func adjustUserRating(username string, delta int) error {
	url := ratingServiceURL + "/api/v1/rating/adjust"
	body, err := json.Marshal(map[string]interface{}{
		"username": username,
		"delta":    delta,
	})
	if err != nil {
		return err
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to adjust rating: status %d", resp.StatusCode)
	}

	return nil
}

func isConditionWorse(originalCondition, returnedCondition string) bool {
	conditionOrder := map[string]int{
		"EXCELLENT": 3,
		"GOOD":      2,
		"BAD":       1,
	}

	originalOrder, ok1 := conditionOrder[originalCondition]
	returnedOrder, ok2 := conditionOrder[returnedCondition]

	if !ok1 || !ok2 {
		return false
	}
	return returnedOrder < originalOrder
}

func queueRequestForRetry(method, url string, headers map[string]string, body []byte) {
	retryQueue.Enqueue(&queue.RetryRequest{
		ID:         uuid.New().String(),
		Method:     method,
		URL:        url,
		Headers:    headers,
		Body:       body,
		RetryAt:    time.Now().Add(retryDelay),
		RetryCount: 0,
		MaxRetries: maxRetries,
	})
}
