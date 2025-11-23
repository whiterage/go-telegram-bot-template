package constants

import "testing"

func TestMainWorkTypes(t *testing.T) {
	// Test that MainWorkTypes is not empty and has expected structure
	if len(MainWorkTypes) == 0 {
		t.Error("MainWorkTypes should not be empty")
	}

	// Check that all types are non-empty strings
	for i, workType := range MainWorkTypes {
		if workType == "" {
			t.Errorf("MainWorkTypes[%d] should not be empty", i)
		}
	}
}

func TestExtraWorkTypes(t *testing.T) {
	// Test that ExtraWorkTypes is not empty and has expected structure
	if len(ExtraWorkTypes) == 0 {
		t.Error("ExtraWorkTypes should not be empty")
	}

	// Check that all types are non-empty strings
	for i, workType := range ExtraWorkTypes {
		if workType == "" {
			t.Errorf("ExtraWorkTypes[%d] should not be empty", i)
		}
	}
}

func TestMaxFileSize(t *testing.T) {
	expected := 20 * 1024 * 1024 // 20 МБ
	if MaxFileSize != expected {
		t.Errorf("MaxFileSize = %d, want %d", MaxFileSize, expected)
	}
}

func TestAllowedMimeTypes(t *testing.T) {
	allowedTypes := []string{
		"application/pdf",
		"image/jpeg",
		"image/jpg",
		"image/png",
		"application/msword",
		"application/vnd.openxmlformats-officedocument.wordprocessingml.document",
	}

	for _, mimeType := range allowedTypes {
		if !AllowedMimeTypes[mimeType] {
			t.Errorf("MimeType %q should be allowed", mimeType)
		}
	}

	// Проверяем, что неразрешенные типы не допускаются
	disallowedTypes := []string{
		"application/zip",
		"text/plain",
		"video/mp4",
	}

	for _, mimeType := range disallowedTypes {
		if AllowedMimeTypes[mimeType] {
			t.Errorf("MimeType %q should not be allowed", mimeType)
		}
	}
}

func TestProfilePageSize(t *testing.T) {
	if ProfilePageSize != 5 {
		t.Errorf("ProfilePageSize = %d, want 5", ProfilePageSize)
	}
}

func TestMaxTrackedMessages(t *testing.T) {
	if MaxTrackedMessages != 20 {
		t.Errorf("MaxTrackedMessages = %d, want 20", MaxTrackedMessages)
	}
}

