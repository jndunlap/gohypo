package api

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/tidwall/gjson"
	"gohypo/domain/core"
)

// APIReader handles fetching data from REST API endpoints
type APIReader struct {
	config       *APIDataSource
	httpClient   *http.Client
	rateLimiter  *RateLimiter
}

// NewAPIReader creates a new API reader for a data source
func NewAPIReader(config *APIDataSource) *APIReader {
	return &APIReader{
		config: config,
		httpClient: &http.Client{
			Timeout: config.Timeout,
		},
		rateLimiter: NewRateLimiter(config.RateLimit),
	}
}

// FetchData retrieves data from the configured API endpoint
func (r *APIReader) FetchData(ctx context.Context) (*APIData, error) {
	startTime := time.Now()

	// Rate limiting check
	if err := r.rateLimiter.Wait(ctx); err != nil {
		return nil, fmt.Errorf("rate limit exceeded: %w", err)
	}

	var allRecords []map[string]interface{}
	var finalMetadata APIMetadata
	cursor := ""
	page := 0

	for page < r.config.MaxPages {
		// Build request URL with pagination
		url := r.buildURL(cursor, page)

		// Make HTTP request
		req, err := r.buildRequest(ctx, url)
		if err != nil {
			return nil, fmt.Errorf("failed to build request: %w", err)
		}

		reqStart := time.Now()
		resp, err := r.httpClient.Do(req)
		reqDuration := time.Since(reqStart)

		if err != nil {
			return nil, fmt.Errorf("HTTP request failed: %w", err)
		}

		// Read response
		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()

		if err != nil {
			return nil, fmt.Errorf("failed to read response: %w", err)
		}

		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(body))
		}

		// Parse records from response
		records, pageMeta, err := r.parseResponse(body, reqDuration, resp)
		if err != nil {
			return nil, fmt.Errorf("failed to parse response: %w", err)
		}

		allRecords = append(allRecords, records...)

		// Initialize metadata from first page
		if page == 0 {
			finalMetadata = pageMeta
			finalMetadata.URL = url
			finalMetadata.FetchedAt = startTime
		}

		// Check pagination continuation
		if !r.hasMorePages(pageMeta, page) {
			break
		}

		cursor = r.extractNextCursor(body)
		page++

		// Rate limiting between pages
		time.Sleep(time.Minute / time.Duration(r.config.RateLimit))
	}

	finalMetadata.ResponseTime = time.Since(startTime)
	finalMetadata.RecordsCount = len(allRecords)

	return &APIData{
		Source:      r.config,
		RawResponse: []byte{}, // Could store compressed version
		ParsedData:  allRecords,
		Metadata:    finalMetadata,
	}, nil
}

// buildURL constructs the request URL with pagination parameters
func (r *APIReader) buildURL(cursor string, page int) string {
	url := r.config.BaseURL

	// Add query parameters
	params := make([]string, 0, len(r.config.QueryParams))

	// Add configured query params
	for k, v := range r.config.QueryParams {
		params = append(params, fmt.Sprintf("%s=%s", k, v))
	}

	// Add pagination
	switch r.config.PaginationType {
	case "offset":
		offset := page * r.config.PageSize
		params = append(params, fmt.Sprintf("offset=%d", offset))
		params = append(params, fmt.Sprintf("limit=%d", r.config.PageSize))
	case "page":
		params = append(params, fmt.Sprintf("page=%d", page+1))
		params = append(params, fmt.Sprintf("per_page=%d", r.config.PageSize))
	case "cursor":
		if cursor != "" {
			params = append(params, fmt.Sprintf("cursor=%s", cursor))
		}
	}

	if len(params) > 0 {
		url += "?" + strings.Join(params, "&")
	}

	return url
}

// buildRequest creates an HTTP request with authentication
func (r *APIReader) buildRequest(ctx context.Context, url string) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	// Add headers
	for k, v := range r.config.Headers {
		req.Header.Set(k, v)
	}

	// Add authentication
	switch r.config.AuthMethod {
	case "bearer":
		req.Header.Set("Authorization", "Bearer "+r.config.AuthToken)
	case "api_key":
		req.Header.Set("X-API-Key", r.config.AuthToken)
	case "basic":
		req.SetBasicAuth(r.config.Username, r.config.Password)
	}

	return req, nil
}

// parseResponse extracts records from JSON response
func (r *APIReader) parseResponse(body []byte, responseTime time.Duration, resp *http.Response) ([]map[string]interface{}, APIMetadata, error) {
	// Extract data array using JSONPath
	dataPath := r.config.DataPath
	if dataPath == "" {
		dataPath = "."
	}

	dataResult := gjson.GetBytes(body, dataPath)
	if !dataResult.Exists() {
		return nil, APIMetadata{}, fmt.Errorf("data path '%s' not found in response", dataPath)
	}

	var records []map[string]interface{}

	// Handle different JSON structures
	if dataResult.IsArray() {
		// Direct array of objects
		if err := json.Unmarshal([]byte(dataResult.Raw), &records); err != nil {
			return nil, APIMetadata{}, fmt.Errorf("failed to parse data array: %w", err)
		}
	} else if dataResult.IsObject() {
		// Single object - wrap in array
		var singleRecord map[string]interface{}
		if err := json.Unmarshal([]byte(dataResult.Raw), &singleRecord); err != nil {
			return nil, APIMetadata{}, fmt.Errorf("failed to parse data object: %w", err)
		}
		records = []map[string]interface{}{singleRecord}
	} else {
		return nil, APIMetadata{}, fmt.Errorf("data path '%s' is not an array or object", dataPath)
	}

	// Extract rate limit info from headers
	metadata := APIMetadata{
		StatusCode:   resp.StatusCode,
		ResponseTime: responseTime,
		ContentType:  resp.Header.Get("Content-Type"),
	}

	if remaining := resp.Header.Get("X-RateLimit-Remaining"); remaining != "" {
		if val, err := strconv.Atoi(remaining); err == nil {
			metadata.RateLimitRemaining = val
		}
	}

	if reset := resp.Header.Get("X-RateLimit-Reset"); reset != "" {
		if val, err := strconv.ParseInt(reset, 10, 64); err == nil {
			metadata.RateLimitReset = time.Unix(val, 0)
		}
	}

	return records, metadata, nil
}

// hasMorePages determines if there are more pages to fetch
func (r *APIReader) hasMorePages(metadata APIMetadata, currentPage int) bool {
	if r.config.PaginationType == "none" {
		return false
	}

	// Check rate limit remaining
	if metadata.RateLimitRemaining > 0 && metadata.RateLimitRemaining < 10 {
		// Conservative: stop if we have less than 10 requests remaining
		return false
	}

	// Check page limit
	if currentPage+1 >= r.config.MaxPages {
		return false
	}

	// Check for pagination indicators in response
	// This would be expanded based on specific API patterns
	return true
}

// extractNextCursor extracts cursor for next page
func (r *APIReader) extractNextCursor(body []byte) string {
	// Common cursor field names
	cursorFields := []string{"next_cursor", "cursor", "next", "continuation_token"}

	for _, field := range cursorFields {
		if cursor := gjson.GetBytes(body, field); cursor.Exists() && cursor.String() != "" {
			return cursor.String()
		}
	}

	return ""
}

// ComputeSchemaFingerprint generates a statistical fingerprint of the data
func (r *APIReader) ComputeSchemaFingerprint(data []map[string]interface{}) (*SchemaFingerprint, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("no data to fingerprint")
	}

	fingerprint := &SchemaFingerprint{
		Version:   1,
		Fields:    make(map[string]FieldFingerprint),
		CreatedAt: time.Now(),
	}

	// Sample subset for fingerprinting (up to 1000 records)
	sampleSize := len(data)
	if sampleSize > 1000 {
		sampleSize = 1000
	}

	// Collect field data
	fieldData := make(map[string][]interface{})
	for i := 0; i < sampleSize; i++ {
		for fieldName, value := range data[i] {
			fieldData[fieldName] = append(fieldData[fieldName], value)
		}
	}

	// Compute fingerprint for each field
	for fieldName, values := range fieldData {
		fp, err := r.computeFieldFingerprint(fieldName, values)
		if err != nil {
			continue // Skip fields we can't fingerprint
		}
		fingerprint.Fields[fieldName] = *fp
	}

	// Compute data hash for change detection
	dataHash := r.computeDataHash(data[:min(100, len(data))])
	fingerprint.DataHash = dataHash

	return fingerprint, nil
}

// computeFieldFingerprint calculates statistical properties of a field
func (r *APIReader) computeFieldFingerprint(fieldName string, values []interface{}) (*FieldFingerprint, error) {
	fp := &FieldFingerprint{
		SampleSize: len(values),
	}

	// Determine data type from sample
	fp.DataType = r.inferDataType(values)

	// Check nullability
	nullCount := 0
	numericValues := []float64{}

	for _, val := range values {
		if val == nil {
			nullCount++
			continue
		}

		// Convert to numeric for statistical calculations
		if numVal := r.toFloat64(val); !math.IsNaN(numVal) {
			numericValues = append(numericValues, numVal)
		}
	}

	fp.Nullable = nullCount > 0

	// Store sample values (up to 10)
	sampleCount := min(10, len(values))
	fp.SampleValues = make([]interface{}, 0, sampleCount)
	for i := 0; i < sampleCount; i++ {
		fp.SampleValues = append(fp.SampleValues, values[i])
	}

	// Compute statistics if we have numeric data
	if len(numericValues) > 0 {
		fp.Mean = r.mean(numericValues)
		fp.StdDev = r.stdDev(numericValues, fp.Mean)
		fp.Min = r.min(numericValues)
		fp.Max = r.max(numericValues)
		fp.Kurtosis = r.kurtosis(numericValues, fp.Mean, fp.StdDev)
		fp.Entropy = r.entropy(numericValues)
	}

	// Compute cardinality (unique values)
	uniqueValues := make(map[string]bool)
	for _, val := range values {
		uniqueValues[fmt.Sprintf("%v", val)] = true
	}
	fp.Cardinality = len(uniqueValues)

	return fp, nil
}

// Helper functions for statistical calculations
func (r *APIReader) inferDataType(values []interface{}) string {
	if len(values) == 0 {
		return "unknown"
	}

	hasString := false
	hasNumber := false
	hasBool := false

	for _, val := range values {
		if val == nil {
			continue
		}

		switch val.(type) {
		case string:
			hasString = true
		case float64, int, int64:
			hasNumber = true
		case bool:
			hasBool = true
		}
	}

	if hasString && !hasNumber && !hasBool {
		return "string"
	}
	if hasNumber && !hasString && !hasBool {
		return "numeric"
	}
	if hasBool && !hasString && !hasNumber {
		return "boolean"
	}

	return "mixed"
}

func (r *APIReader) toFloat64(val interface{}) float64 {
	switch v := val.(type) {
	case float64:
		return v
	case int:
		return float64(v)
	case int64:
		return float64(v)
	default:
		return math.NaN()
	}
}

func (r *APIReader) mean(values []float64) float64 {
	sum := 0.0
	for _, v := range values {
		sum += v
	}
	return sum / float64(len(values))
}

func (r *APIReader) stdDev(values []float64, mean float64) float64 {
	sumSquares := 0.0
	for _, v := range values {
		sumSquares += (v - mean) * (v - mean)
	}
	return math.Sqrt(sumSquares / float64(len(values)))
}

func (r *APIReader) min(values []float64) float64 {
	min := values[0]
	for _, v := range values[1:] {
		if v < min {
			min = v
		}
	}
	return min
}

func (r *APIReader) max(values []float64) float64 {
	max := values[0]
	for _, v := range values[1:] {
		if v > max {
			max = v
		}
	}
	return max
}

func (r *APIReader) kurtosis(values []float64, mean, stdDev float64) float64 {
	if stdDev == 0 || len(values) < 4 {
		return 0
	}

	sumFourth := 0.0
	for _, v := range values {
		dev := (v - mean) / stdDev
		sumFourth += dev * dev * dev * dev
	}

	n := float64(len(values))
	return (sumFourth / n) - 3 // Excess kurtosis
}

func (r *APIReader) entropy(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}

	// Simple entropy calculation based on value distribution
	// Bin values into categories
	bins := make(map[string]int)
	for _, v := range values {
		// Simple binning by rounding to nearest integer
		bin := fmt.Sprintf("%.0f", v)
		bins[bin]++
	}

	entropy := 0.0
	n := float64(len(values))
	for _, count := range bins {
		p := float64(count) / n
		if p > 0 {
			entropy -= p * math.Log2(p)
		}
	}

	return entropy
}

func (r *APIReader) computeDataHash(sampleData []map[string]interface{}) string {
	jsonBytes, _ := json.Marshal(sampleData)
	hash := sha256.Sum256(jsonBytes)
	return fmt.Sprintf("%x", hash)
}

// RateLimiter implements token bucket rate limiting
type RateLimiter struct {
	rate       int           // requests per minute
	tokens     chan struct{}
	resetTimer *time.Timer
}

func NewRateLimiter(requestsPerMinute int) *RateLimiter {
	rl := &RateLimiter{
		rate:   requestsPerMinute,
		tokens: make(chan struct{}, requestsPerMinute),
	}

	// Fill initial tokens
	for i := 0; i < requestsPerMinute; i++ {
		select {
		case rl.tokens <- struct{}{}:
		default:
		}
	}

	// Reset tokens every minute
	rl.resetTimer = time.AfterFunc(time.Minute, rl.resetTokens)
	return rl
}

func (rl *RateLimiter) Wait(ctx context.Context) error {
	select {
	case <-rl.tokens:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (rl *RateLimiter) resetTokens() {
	// Drain existing tokens
	for len(rl.tokens) > 0 {
		<-rl.tokens
	}

	// Add new tokens
	for i := 0; i < rl.rate; i++ {
		select {
		case rl.tokens <- struct{}{}:
		default:
		}
	}

	// Schedule next reset
	rl.resetTimer.Reset(time.Minute)
}
