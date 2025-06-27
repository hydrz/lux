package gaodun

import (
	"testing"
)

func TestExtractCourseID(t *testing.T) {
	tests := []struct {
		name     string
		url      string
		expected string
		hasError bool
	}{
		{
			name:     "course_id parameter",
			url:      "https://gaodun.com/course?course_id=17244",
			expected: "17244",
			hasError: false,
		},
		{
			name:     "courseId parameter",
			url:      "https://gaodun.com/course?courseId=17244",
			expected: "17244",
			hasError: false,
		},
		{
			name:     "course path",
			url:      "https://gaodun.com/course/17244",
			expected: "17244",
			hasError: false,
		},
		{
			name:     "invalid URL",
			url:      "https://gaodun.com/invalid",
			expected: "",
			hasError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := extractCourseID(tt.url)

			if tt.hasError {
				if err == nil {
					t.Errorf("Expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error but got: %v", err)
				}
				if result != tt.expected {
					t.Errorf("Expected %s but got %s", tt.expected, result)
				}
			}
		})
	}
}

func TestSanitizeFileName(t *testing.T) {
	e := &extractor{}

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "normal filename",
			input:    "Normal File Name",
			expected: "Normal File Name",
		},
		{
			name:     "invalid characters",
			input:    "File/With\\Invalid:Characters*?\"<>|",
			expected: "File-With-Invalid-Characters------",
		},
		{
			name:     "spaces and dots",
			input:    "   File with spaces   ",
			expected: "File with spaces",
		},
		{
			name:     "long filename",
			input:    "Very long file name that exceeds the maximum allowed length for filesystem compatibility and should be truncated to avoid issues",
			expected: "Very long file name that exceeds the maximum allowed length for filesystem compatibility and should",
		},
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := e.sanitizeFileName(tt.input)
			if result != tt.expected {
				t.Errorf("Expected '%s', got '%s'", tt.expected, result)
			}
		})
	}
}

func TestIsGStudyCourse(t *testing.T) {
	client := NewClient()

	ex := &extractor{
		api: client,
	}

	b1, err := ex.isGStudyCourse("17244")
	if err != nil {
		t.Errorf("isGStudyCourse returned an error: %v", err)
	}
	if b1 {
		t.Errorf("isGStudyCourse expected false, got true")
	}

	b2, err := ex.isGStudyCourse("33795")

	if err != nil {
		t.Errorf("isGStudyCourse returned an error: %v", err)
	}
	if !b2 {
		t.Errorf("isGStudyCourse expected true, got false")
	}

}
