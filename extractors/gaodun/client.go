package gaodun

import (
	"compress/gzip"
	"crypto/rand"
	"encoding/json"
	"maps"

	"fmt"
	"io"
	"log"
	"os"
	"strings"

	"github.com/hydrz/lux/request"
)

const (
	// API endpoints
	endpoint = "https://apigateway.gaodun.com"

	// Headers required for API requests
	userAgent  = "GdClient/10.0.81 Android/14 H2OS/110_14.0.0.630(cn01) GdNetwork/1.0.5"
	ApiVersion = "264"
)

// APIClient defines the interface for Gaodun API operations
type APIClient interface {
	// GStudy retrieves syllabus information for g-study courses
	// GET https://apigateway.gaodun.com/g-study/api/v1/front/course/{courseID}/gradation/syllabus
	GStudy(courseID string) ([]Gradation, error)

	// Glive retrieves detailed syllabus for glive courses
	// GET https://apigateway.gaodun.com/g-study/api/v1/front/course/{courseID}/syllabus/glive/{syllabusID}
	GStudySyllabus(courseID, syllabusID string) (*Syllabus, error)

	// GLiveCheck get real video ID for glive vod
	GLiveCheck(roomId, token string) (string, error)

	// EpStudy retrieves gradation for ep-study courses
	// GET https://apigateway.gaodun.com/ep-study/front/course/{courseID}/gradation
	EpStudy(courseID string) ([]Gradation, error)

	// EpStudySyllabus retrieves detailed syllabus for ep-study courses
	// GET https://apigateway.gaodun.com/ep-study/front/course/{courseID}/syllabus/{syllabusID}
	EpStudySyllabus(courseID, syllabusID string) ([]Syllabus, error)

	// VideoResource retrieves video stream information
	// GET https://apigateway.gaodun.com/glive2-vod/api/v1/live/resource?code={code}&res={res}&channel={channel}
	VideoResource(code, res string, channel int) (*VideoResource, error)

	// AuthorizeKey retrieves authorization key for video access
	// GET https://apigateway.gaodun.com/glive2-vod/api/v1/replay/authorizeKey?id={id}
	AuthorizeKey(id string) (string, error)

	Headers() map[string]string
}

// NewClient creates a new API client with proper authentication headers
func NewClient() APIClient {
	token := os.Getenv("GAODUN_AUTH_TOKEN")
	if token == "" {
		log.Fatal("GAODUN_AUTH_TOKEN environment variable is not set")
	}

	xRequestedExtend := fmt.Sprintf(
		`{"apiConfigVersion":"%s","appStore":"%s","appVersion":"%s","phoneBrand":"%s","appScheme":"%s","deviceId":"%s","appChannel":"%s","appChannelName":"%s"}`,
		ApiVersion, "oppo", "264", "oneplus", "gaodunapp", generateDeviceID(), "oppo", "android",
	)
	return &client{
		headers: map[string]string{
			"User-Agent":         userAgent,
			"Authentication":     token,
			"ApiVersion":         ApiVersion,
			"X-Requested-Extend": xRequestedExtend,
			"Host":               "apigateway.gaodun.com",
			"Connection":         "Keep-Alive",
			"Accept-Encoding":    "gzip",
		},
	}
}

// client handles all API interactions with Gaodun services
type client struct {
	headers map[string]string
}

// Headers returns the headers used for API requests
func (c *client) Headers() map[string]string {
	return maps.Clone(c.headers)
}

func (c *client) do(method, url string, body io.Reader, additionalHeaders ...map[string]string) ([]byte, error) {
	headers := maps.Clone(c.headers)

	// Merge additional headers if provided
	if len(additionalHeaders) > 0 {
		for k, v := range additionalHeaders[0] {
			headers[k] = v
		}
	}

	resp, err := request.Request(method, url, body, headers)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}

	// Check if the response is compressed
	if strings.Contains(resp.Header.Get("Content-Encoding"), "gzip") {
		// If gzip is used, we need to decompress the response body
		gzipReader, err := gzip.NewReader(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to create gzip reader: %w", err)
		}
		defer gzipReader.Close()
		resp.Body = gzipReader
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}
	defer resp.Body.Close()
	if strings.Contains(string(bodyBytes), "登录超时") {
		return nil, fmt.Errorf("login timeout, please check your authentication token")
	}

	return bodyBytes, nil
}

// GStudy retrieves syllabus information for g-study courses
func (c *client) GStudy(courseID string) ([]Gradation, error) {
	url := fmt.Sprintf("%s/g-study/api/v1/front/course/%s/gradation/syllabus", endpoint, courseID)

	resp, err := c.do("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get glive syllabus: %w", err)
	}

	var apiResp APIResponse[[]Gradation]
	if err := json.Unmarshal(resp, &apiResp); err != nil {
		return nil, fmt.Errorf("failed to parse glive syllabus response: %w", err)
	}

	if apiResp.Status != 0 {
		return nil, fmt.Errorf("API error: %s", apiResp.Message)
	}

	return apiResp.Result, nil
}

// GStudySyllabus retrieves detailed syllabus for glive courses
func (c *client) GStudySyllabus(courseID, syllabusID string) (*Syllabus, error) {
	url := fmt.Sprintf("%s/g-study/api/v1/front/course/%s/syllabus/glive/%s", endpoint, courseID, syllabusID)

	resp, err := c.do("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get glive syllabus: %w", err)
	}

	var apiResp APIResponse[Syllabus]
	if err := json.Unmarshal(resp, &apiResp); err != nil {
		return nil, fmt.Errorf("failed to parse glive syllabus response: %w", err)
	}

	if apiResp.Status != 0 {
		return nil, fmt.Errorf("API error: %s", apiResp.Message)
	}

	return &apiResp.Result, nil
}

// EpStudy retrieves gradation for ep-study courses
func (c *client) EpStudy(courseID string) ([]Gradation, error) {
	url := fmt.Sprintf("%s/ep-study/front/course/%s/gradation", endpoint, courseID)

	resp, err := c.do("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get glive syllabus: %w", err)
	}

	var apiResp APIResponse[[]Gradation]
	if err := json.Unmarshal(resp, &apiResp); err != nil {
		return nil, fmt.Errorf("failed to parse ep gradation response: %w", err)
	}

	if apiResp.Status != 0 {
		return nil, fmt.Errorf("API error: %s", apiResp.Message)
	}

	return apiResp.Result, nil
}

// EpStudySyllabus retrieves detailed syllabus for ep-study courses
func (c *client) EpStudySyllabus(courseID, syllabusID string) ([]Syllabus, error) {
	url := fmt.Sprintf("%s/ep-study/front/course/%s/syllabus/%s?show_own_teacher=true", endpoint, courseID, syllabusID)

	resp, err := c.do("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get glive syllabus: %w", err)
	}

	var apiResp APIResponse[map[string]interface{}]
	if err := json.Unmarshal(resp, &apiResp); err != nil {
		return nil, fmt.Errorf("failed to parse ep syllabus response: %w", err)
	}

	if apiResp.Status != 0 {
		return nil, fmt.Errorf("API error: %s", apiResp.Message)
	}

	// The ep syllabus has a different structure with "items" field
	items, ok := apiResp.Result["items"]
	if !ok {
		return nil, fmt.Errorf("no items found in ep syllabus response")
	}

	// Convert to our SyllabusResponse structure
	itemsBytes, err := json.Marshal(items)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal items: %w", err)
	}

	var syllabusItems []Syllabus
	if err := json.Unmarshal(itemsBytes, &syllabusItems); err != nil {
		return nil, fmt.Errorf("failed to unmarshal syllabus items: %w", err)
	}

	return syllabusItems, nil
}

// VideoResource retrieves video stream information
func (c *client) VideoResource(code, res string, channel int) (*VideoResource, error) {
	url := fmt.Sprintf("%s/glive2-vod/api/v1/live/resource?code=%s&res=%s&channel=%d", endpoint, code, res, channel)
	resp, err := c.do("GET", url, nil, map[string]string{
		"isLiveVodAuthenticate": "true",
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get glive syllabus: %w", err)
	}

	var apiResp APIResponse[VideoResource]
	if err := json.Unmarshal(resp, &apiResp); err != nil {
		return nil, fmt.Errorf("failed to parse video resource response: %w", err)
	}

	if apiResp.Status != 200 {
		return nil, fmt.Errorf("API error: %s", apiResp.Message)
	}

	return &apiResp.Result, nil
}

func (c *client) GLiveCheck(roomId, token string) (string, error) {
	url := fmt.Sprintf("%s/glive2-vod/api/v1/vod/check?roomId=%s&token=%s", endpoint, roomId, token)

	resp, err := c.do("GET", url, nil, map[string]string{
		"isLiveVodAuthenticate": "true",
	})
	if err != nil {
		return "", fmt.Errorf("failed to check glive vod: %w", err)
	}

	type result struct {
		Code string `json:"code"`
	}

	var apiResp APIResponse[result]
	if err := json.Unmarshal(resp, &apiResp); err != nil {
		return "", fmt.Errorf("failed to parse glive check response: %w", err)
	}
	if apiResp.Status != 200 {
		return "", fmt.Errorf("API error: %s", apiResp.Message)
	}
	return apiResp.Result.Code, nil
}

// AuthorizeKey retrieves authorization key for video access
func (c *client) AuthorizeKey(id string) (string, error) {
	url := fmt.Sprintf("%s/glive2-vod/api/v1/replay/authorizeKey?id=%s", endpoint, id)

	resp, err := c.do("GET", url, nil)
	if err != nil {
		return "", fmt.Errorf("failed to get glive syllabus: %w", err)
	}

	return string(resp), nil
}

// generateDeviceID generates a random device ID for API requests
func generateDeviceID() string {
	b := make([]byte, 33)
	_, err := rand.Read(b)
	if err != nil {
		return "24ca6c8e5eed9334b822c28eda895e70a" // Default value if random generation fails
	}
	b[0] = '2' // Ensure it starts with '2' to match the expected format
	return fmt.Sprintf("%x", b)
}
