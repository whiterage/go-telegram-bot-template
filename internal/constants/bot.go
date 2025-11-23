package constants

// WorkTypes contains service types for the form
// These are template values - customize them for your specific use case
var (
	MainWorkTypes = []string{
		"Услуга 1",
		"Услуга 2",
		"Услуга 3",
		"Услуга 4",
		"Услуга 5",
		"Услуга 6",
		"Услуга 7",
		"Услуга 8",
		"Услуга 9",
	}
	ExtraWorkTypes = []string{
		"Дополнительная услуга 1",
		"Дополнительная услуга 2",
		"Дополнительная услуга 3",
	}
)

// FileValidation константы для валидации файлов
const (
	MaxFileSize = 20 * 1024 * 1024 // 20 МБ в байтах
)

// AllowedMimeTypes разрешенные типы файлов для загрузки
var AllowedMimeTypes = map[string]bool{
	"application/pdf":    true,
	"image/jpeg":         true,
	"image/jpg":          true,
	"image/png":          true,
	"application/msword": true,
	"application/vnd.openxmlformats-officedocument.wordprocessingml.document": true,
}

// ProfilePageSize размер страницы для пагинации профиля
const ProfilePageSize = 5

// MaxTrackedMessages максимальное количество отслеживаемых сообщений
const MaxTrackedMessages = 20

