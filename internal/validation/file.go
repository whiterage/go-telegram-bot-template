package validation

import (
	"fmt"
	"strings"
)

const (
	MaxFileSize = 20 * 1024 * 1024 // 20MB
)

var AllowedMimeTypes = map[string]bool{
	"image/jpeg":      true,
	"image/jpg":       true,
	"image/png":       true,
	"application/pdf": true,
}

var AllowedExtensions = map[string]bool{
	".jpg":  true,
	".jpeg": true,
	".png":  true,
	".pdf":  true,
}

type FileValidationError struct {
	Message string
}

func (e FileValidationError) Error() string {
	return e.Message
}

func ValidateReceiptFile(fileName string, fileSize int64, mimeType string) error {
	// Проверка размера файла
	if fileSize > MaxFileSize {
		return FileValidationError{
			Message: fmt.Sprintf("Файл слишком большой. Максимальный размер: %d МБ", MaxFileSize/(1024*1024)),
		}
	}

	// Проверка расширения файла
	fileName = strings.ToLower(fileName)
	hasValidExtension := false
	for ext := range AllowedExtensions {
		if strings.HasSuffix(fileName, ext) {
			hasValidExtension = true
			break
		}
	}

	if !hasValidExtension {
		return FileValidationError{
			Message: "Неподдерживаемый формат файла. Разрешены: JPG, PNG, PDF",
		}
	}

	// Проверка MIME типа (если доступен)
	if mimeType != "" {
		if !AllowedMimeTypes[mimeType] {
			return FileValidationError{
				Message: "Неподдерживаемый тип файла. Разрешены: JPG, PNG, PDF",
			}
		}
	}

	return nil
}

func GetFileTypeFromName(fileName string) string {
	fileName = strings.ToLower(fileName)
	if strings.HasSuffix(fileName, ".pdf") {
		return "document"
	}
	return "photo"
}
