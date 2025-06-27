package gaodun

import (
	"fmt"
	"log/slog"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"github.com/hydrz/lux/extractors"
	"github.com/hydrz/lux/request"
	"github.com/hydrz/lux/utils"
	"github.com/pkg/errors"
)

func init() {
	extractors.Register("gaodun", New())
}

type extractor struct {
	api    APIClient
	option extractors.Options
}

// New returns a new gaodun extractor
func New() extractors.Extractor {
	return &extractor{}
}

func (e *extractor) isGStudyCourse(courseID string) (bool, error) {
	gs, err := e.api.GStudy(courseID)
	if err != nil {
		return false, errors.WithStack(err)
	}

	if len(gs) == 0 {
		return false, errors.New("no G-Study course found")
	}

	g := gs[0] // Use the first gradation as representative
	if g.GSyllabus != nil && g.EpSyllabus == nil {
		return true, nil // G-Study course has syllabus, no Ep-Study syllabus
	}

	return false, nil // Not a G-Study course, might be Ep-Study
}

// extractCourseID extracts course ID from the URL
func extractCourseID(URL string) (string, error) {
	// Support both course_id and courseId parameters
	courseIDPattern := regexp.MustCompile(`(?:course_id|courseId)=(\d+)`)
	matches := courseIDPattern.FindStringSubmatch(URL)

	if len(matches) >= 2 {
		return matches[1], nil
	}

	// Also support course ID in URL path like /course/17244
	pathPattern := regexp.MustCompile(`/course/(\d+)`)
	matches = pathPattern.FindStringSubmatch(URL)

	if len(matches) >= 2 {
		return matches[1], nil
	}

	return "", errors.New("course ID not found in URL")
}

// Extract extracts video and PDF data from gaodun.com URLs
func (e *extractor) Extract(URL string, option extractors.Options) ([]*extractors.Data, error) {
	// Create API client
	e.api = NewClient()
	e.option = option

	// Set request options for better compatibility with Gaodun servers
	request.SetOptions(request.Options{
		UserAgent:  "Mozilla/5.0 (Linux; Android 10; SM-G973F) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.120 Mobile Safari/537.36",
		Refer:      "https://www.gaodun.com/",
		RetryTimes: 3,
	})

	// Parse course ID from URL
	courseID, err := extractCourseID(URL)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	isGStudy, err := e.isGStudyCourse(courseID)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	// Determine course type and extract data accordingly
	var allData []*extractors.Data

	if isGStudy {
		allData, err = e.extractGStudyCourse(courseID)
	} else {
		allData, err = e.extractEpStudyCourse(courseID)
	}

	if err != nil {
		return nil, errors.WithStack(err)
	}

	return allData, nil
}

// sanitizeFileName sanitizes file names to be safe for filesystem
func (e *extractor) sanitizeFileName(name string) string {
	// Replace invalid characters with safe alternatives
	replacements := map[string]string{
		"/":  "-",
		"\\": "-",
		":":  "-",
		"*":  "-",
		"?":  "-",
		"\"": "-",
		"<":  "-",
		">":  "-",
		"|":  "-",
	}

	result := name
	for old, new := range replacements {
		result = strings.ReplaceAll(result, old, new)
	}

	// Trim spaces and dots from the beginning and end
	result = strings.Trim(result, " .")

	// Limit length to avoid filesystem issues
	if len(result) > 100 {
		result = result[:100]
		// Trim again after truncation
		result = strings.TrimRight(result, " .")
	}

	return result
}

// extractGStudyCourse extracts data from G-Study courses
func (e *extractor) extractGStudyCourse(courseID string) ([]*extractors.Data, error) {
	// Get gradation information
	gradations, err := e.api.GStudy(courseID)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	var allData []*extractors.Data
	var mu sync.Mutex
	var wg sync.WaitGroup

	// Process each gradation concurrently
	for _, gradation := range gradations {
		wg.Add(1)
		go func(grad Gradation) {
			defer wg.Done()

			if grad.GSyllabus == nil {
				return
			}

			syllabus, err := e.api.GStudySyllabus(courseID, grad.SyllabusID.String())
			if err != nil || syllabus == nil {
				slog.Error("failed to get G-Study syllabus",
					"course_id", courseID,
					"gradation_name", grad.Name,
					"error", err,
				)
				return // Skip this gradation if syllabus retrieval fails
			}

			// Extract data from this gradation's syllabus
			data, err := e.extractGStudySyllabus(courseID, grad.Name, *syllabus)
			if err != nil {
				// Log error but continue with other gradations
				slog.Error("failed to extract G-Study syllabus items",
					"course_id", courseID,
					"gradation_name", grad.Name,
					"error", err,
				)
				return
			}

			mu.Lock()
			allData = append(allData, data...)
			mu.Unlock()
		}(gradation)
	}

	wg.Wait()
	return allData, nil
}

// extractEpStudyCourse extracts data from Ep-Study courses
func (e *extractor) extractEpStudyCourse(courseID string) ([]*extractors.Data, error) {
	// Get gradation information
	gradations, err := e.api.EpStudy(courseID)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	var allData []*extractors.Data
	var mu sync.Mutex
	var wg sync.WaitGroup

	// Process each gradation concurrently
	for _, gradation := range gradations {
		wg.Add(1)
		go func(grad Gradation) {
			defer wg.Done()
			// Get detailed syllabus for this gradation
			syllabusID := grad.ID.String()
			if syllabusID == "" {
				return
			}

			syllabus, err := e.api.EpStudySyllabus(courseID, syllabusID)
			if err != nil {
				return
			}

			// Extract data from syllabus items
			data, err := e.extractEpStudySyllabus(courseID, grad.Name, syllabus)
			if err != nil {
				return
			}

			mu.Lock()
			allData = append(allData, data...)
			mu.Unlock()
		}(gradation)
	}

	wg.Wait()
	return allData, nil
}

// extractGStudySyllabus extracts resources from G-Study syllabus
func (e *extractor) extractGStudySyllabus(courseID, gradationName string, syllabus Syllabus) ([]*extractors.Data, error) {
	var allData []*extractors.Data

	// Process children syllabi recursively
	if len(syllabus.Children) > 0 {
		var wg sync.WaitGroup
		var mu sync.Mutex

		for _, child := range syllabus.Children {
			wg.Add(1)
			go func(childSyl Syllabus) {
				defer wg.Done()

				childData, err := e.extractGStudySyllabus(courseID, gradationName, childSyl)
				if err != nil {
					return
				}

				mu.Lock()
				allData = append(allData, childData...)
				mu.Unlock()
			}(child)
		}
		wg.Wait()
	}

	// Extract resources from current syllabus
	baseDir := filepath.Join(courseID, e.sanitizeFileName(gradationName), e.sanitizeFileName(syllabus.Name))

	// Process different types of resources
	resourceGroups := []struct {
		resources []Resource
		subDir    string
	}{
		{syllabus.PreClassResource, "课前"},
		{syllabus.InClassMainResource, "课程"},
		{syllabus.InClassAssistResource, "课辅"},
		{syllabus.AfterClassResource, "课后"},
	}

	var wg sync.WaitGroup
	var mu sync.Mutex

	for _, group := range resourceGroups {
		if len(group.resources) == 0 {
			continue
		}

		wg.Add(1)
		go func(resources []Resource, subDir string) {
			defer wg.Done()

			resourceData, err := e.extractResources(resources, filepath.Join(baseDir))
			if err != nil {
				return
			}

			mu.Lock()
			allData = append(allData, resourceData...)
			mu.Unlock()
		}(group.resources, group.subDir)
	}

	wg.Wait()
	return allData, nil
}

// extractEpStudySyllabus extracts resources from Ep-Study syllabus items
func (e *extractor) extractEpStudySyllabus(courseID, gradationName string, syllabusItems []Syllabus) ([]*extractors.Data, error) {
	var allData []*extractors.Data
	var wg sync.WaitGroup
	var mu sync.Mutex

	for _, item := range syllabusItems {
		wg.Add(1)
		go func(syllabus Syllabus) {
			defer wg.Done()

			// Process children recursively
			if len(syllabus.Children) > 0 {
				childData, err := e.extractEpStudySyllabus(courseID, gradationName, syllabus.Children)
				if err == nil {
					mu.Lock()
					allData = append(allData, childData...)
					mu.Unlock()
				}
			}

			// Skip if this is not a resource item
			if syllabus.IsResource == 0 {
				return
			}

			baseDir := filepath.Join(courseID, e.sanitizeFileName(gradationName), e.sanitizeFileName(syllabus.Name))

			// For Ep-Study, resources might be embedded differently
			// We'll need to extract them based on the syllabus structure
			resourceData, err := e.extractEpStudyResources(syllabus, baseDir)
			if err != nil {
				return
			}

			mu.Lock()
			allData = append(allData, resourceData...)
			mu.Unlock()
		}(item)
	}

	wg.Wait()
	return allData, nil
}

// extractResources processes individual resources and creates extractors.Data
func (e *extractor) extractResources(resources []Resource, baseDir string) ([]*extractors.Data, error) {
	var allData []*extractors.Data
	var wg sync.WaitGroup
	var mu sync.Mutex

	for _, resource := range resources {
		wg.Add(1)
		go func(res Resource) {
			defer wg.Done()

			data, err := e.processResource(res, baseDir)
			if err != nil {
				return
			}

			if data != nil {
				mu.Lock()
				allData = append(allData, data)
				mu.Unlock()
			}
		}(resource)
	}

	wg.Wait()
	return allData, nil
}

// extractEpStudyResources processes Ep-Study specific resources
func (e *extractor) extractEpStudyResources(syllabus Syllabus, baseDir string) ([]*extractors.Data, error) {
	// For Ep-Study courses, we might need to handle resources differently
	// This is a placeholder implementation that can be expanded based on actual data structure
	return []*extractors.Data{}, nil
}

func extractRoomIDAndToken(URL string) (roomID, token string, err error) {
	// Extract room ID and token from gaodunapp:// URL format
	roomIDPattern := regexp.MustCompile(`gaodunapp://gd/liveroom/v2/replays/detail\?recordId=([a-zA-Z0-9]+)&did=[a-zA-Z0-9]+&roomId=([a-zA-Z0-9-]+)&token=([a-zA-Z0-9]+)`)
	matches := roomIDPattern.FindStringSubmatch(URL)

	if len(matches) >= 4 {
		return matches[2], matches[3], nil
	}

	return "", "", errors.New("room ID and token not found in URL")
}

// processResource handles individual resource processing
func (e *extractor) processResource(resource Resource, baseDir string) (*extractors.Data, error) {
	switch resource.Discriminator {
	case "live_new":
		roomID, token, err := extractRoomIDAndToken(resource.LiveUrlPlayBackApp)
		if err != nil {
			slog.Error("failed to extract room ID and token",
				"resource_id", resource.ID,
				"error", err,
			)
			return nil, nil // Skip this resource if extraction fails
		}

		code, err := e.api.GLiveCheck(roomID, token)
		if err != nil {
			slog.Error("failed to check GLive",
				"room_id", roomID,
				"token", token,
				"error", err,
			)
			return nil, nil // Skip this resource if check fails
		}

		// Create a video resource with the extracted code
		resource.VideoID = code
		return e.processVideoResource(resource, baseDir)
	case "video":
		return e.processVideoResource(resource, baseDir)
	case "lecture_note":
		return e.processNonVideoResource(resource, baseDir)
	}

	return nil, nil // Skip unsupported resource types
}

// processVideoResource processes video resources
func (e *extractor) processVideoResource(resource Resource, baseDir string) (*extractors.Data, error) {
	videoID := resource.VideoID

	// Get video resource information
	videoRes, err := e.api.VideoResource(videoID, "SD", 0)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	slog.Info("video resource info",
		"source_id", videoID,
		"title", videoRes.Title,
		"duration", videoRes.Duration,
		"encrypt", videoRes.Encrypt,
		"qualities", len(videoRes.List),
	)

	// Create streams for different qualities
	streams := make(map[string]*extractors.Stream)

	for quality, qualityInfo := range videoRes.List {
		if qualityInfo.Available != 1 || qualityInfo.Path == "" {
			slog.Warn("skipping unavailable quality",
				"quality", quality,
				"available", qualityInfo.Available,
				"has_path", qualityInfo.Path != "",
			)
			continue
		}

		slog.Info("processing video quality",
			"quality", quality,
			"resolution", qualityInfo.Resolution.Resolution,
			"file_size_kb", qualityInfo.FileSize,
			"is_watermark", qualityInfo.IsWatermark,
			"path", qualityInfo.Path,
		)

		urls, err := utils.M3u8URLsWithHeaders(qualityInfo.Path, e.api.Headers())
		if err != nil {
			slog.Error("failed to parse M3U8 URLs",
				"path", qualityInfo.Path,
				"error", err,
			)
			continue
		}

		// Create parts for M3U8 segments
		parts := make([]*extractors.Part, 0, len(urls))
		for i, url := range urls {
			parts = append(parts, &extractors.Part{
				URL: url,
				Ext: "ts",
			})

			// Log first few URLs for debugging
			if i < 3 {
				slog.Debug("TS segment URL", "index", i, "url", url)
			}
		}

		id := resource.VideoID + "_" + quality

		streams[id] = &extractors.Stream{
			ID:      id,
			Parts:   parts,
			Quality: qualityInfo.Resolution.Resolution,
			Size:    int64(qualityInfo.FileSize * 1024),
			NeedMux: false,
		}
	}

	if len(streams) == 0 {
		return nil, errors.New("no available video streams")
	}

	// Create filename with proper extension
	filename := e.sanitizeFileName(resource.Title)
	if filename == "" {
		filename = fmt.Sprintf("video_%d", resource.ID)
	}

	return &extractors.Data{
		Site:    "高顿教育 gaodun.com",
		Title:   filepath.Join(baseDir, filename),
		Type:    extractors.DataTypeVideo,
		Streams: streams,
		URL:     fmt.Sprintf("gaodun://video/%s", videoID),
	}, nil
}

// processNonVideoResource processes non-video resources (PDFs, documents)
func (e *extractor) processNonVideoResource(resource Resource, baseDir string) (*extractors.Data, error) {
	if resource.Path == "" {
		return nil, nil // Skip resources without Path
	}

	// Determine file extension
	ext := resource.Extension
	if ext == "" {
		// Try to determine from MIME type
		switch resource.Mime {
		case "application/pdf":
			ext = "pdf"
		case "application/msword":
			ext = "doc"
		case "application/vnd.openxmlformats-officedocument.wordprocessingml.document":
			ext = "docx"
		default:
			ext = "file"
		}
	}

	// Create filename
	filename := e.sanitizeFileName(resource.Title)
	if filename == "" {
		filename = fmt.Sprintf("document_%d", resource.ID)
	}

	size, err := strconv.Atoi(resource.Filesize)
	if err != nil {
		size = 0 // Default to 0 if filesize is not available
	}

	// Create single part for direct download
	parts := []*extractors.Part{
		{
			URL:  resource.Path,
			Size: int64(size),
			Ext:  ext,
		},
	}

	streams := make(map[string]*extractors.Stream, 1)

	id := strconv.Itoa(resource.ID)
	streams[id] = &extractors.Stream{
		Quality: "Unknown", // Use "Unknown" for non-video resources
		ID:      id,
		Parts:   parts,
		Size:    int64(size),
		Ext:     ext,
	}

	return &extractors.Data{
		Site:    "高顿教育 gaodun.com",
		Title:   filepath.Join(baseDir, filename),
		Type:    extractors.DataTypeDocument, // Use document type for PDFs
		Streams: streams,
		URL:     resource.Path,
	}, nil
}

// testM3U8Accessibility tests if the M3U8 URL is accessible
func (e *extractor) testM3U8Accessibility(m3u8URL string, headers map[string]string) error {
	resp, err := request.Request("HEAD", m3u8URL, nil, headers)
	if err != nil {
		return fmt.Errorf("failed to access M3U8 URL: %w", err)
	}
	defer resp.Body.Close()

	slog.Info("M3U8 accessibility check",
		"url", m3u8URL,
		"status_code", resp.StatusCode,
		"content_type", resp.Header.Get("Content-Type"),
		"content_length", resp.Header.Get("Content-Length"),
	)

	if resp.StatusCode != 200 {
		return fmt.Errorf("M3U8 URL returned status %d", resp.StatusCode)
	}

	return nil
}

// testTSAccessibility tests if a TS segment URL is accessible
func (e *extractor) testTSAccessibility(tsURL string, headers map[string]string) error {
	resp, err := request.Request("HEAD", tsURL, nil, headers)
	if err != nil {
		return fmt.Errorf("failed to access TS URL: %w", err)
	}
	defer resp.Body.Close()

	slog.Info("TS segment accessibility check",
		"url", tsURL,
		"status_code", resp.StatusCode,
		"content_type", resp.Header.Get("Content-Type"),
		"content_length", resp.Header.Get("Content-Length"),
	)

	if resp.StatusCode != 200 {
		return fmt.Errorf("TS URL returned status %d", resp.StatusCode)
	}

	return nil
}
