package ratelimit

import "time"

// ActionType определяет тип действия пользователя
type ActionType int

const (
	ActionMessage ActionType = iota
	ActionFileUpload
	ActionCallback
	ActionCommand
)

// ActionLimits содержит лимиты для разных типов действий
type ActionLimits struct {
	Message    Config
	FileUpload Config
	Callback   Config
	Command    Config
}

// GetDefaultLimits возвращает рекомендуемые лимиты
func GetDefaultLimits() ActionLimits {
	return ActionLimits{
		Message: Config{
			MaxRequests:    100,              // 100 сообщений
			WindowDuration: 15 * time.Minute, // за 15 минут
			RefillRate:     4,                // 4 токена в минуту
			RefillInterval: 15 * time.Second,
		},
		FileUpload: Config{
			MaxRequests:    20, // 20 файлов
			WindowDuration: 20 * time.Minute,
			RefillRate:     1,
			RefillInterval: 2 * time.Minute,
		},
		Callback: Config{
			MaxRequests:    200, // 200 нажатий кнопок
			WindowDuration: 15 * time.Minute,
			RefillRate:     10,
			RefillInterval: 10 * time.Second,
		},
		Command: Config{
			MaxRequests:    20, // 20 команд
			WindowDuration: 10 * time.Minute,
			RefillRate:     1,
			RefillInterval: 30 * time.Second,
		},
	}
}

// GetConservativeLimits возвращает консервативные лимиты (больше защиты)
func GetConservativeLimits() ActionLimits {
	return ActionLimits{
		Message: Config{
			MaxRequests:    50,
			WindowDuration: 5 * time.Minute,
			RefillRate:     2,
			RefillInterval: 30 * time.Second,
		},
		FileUpload: Config{
			MaxRequests:    10,
			WindowDuration: 10 * time.Minute,
			RefillRate:     1,
			RefillInterval: 2 * time.Minute,
		},
		Callback: Config{
			MaxRequests:    100,
			WindowDuration: 5 * time.Minute,
			RefillRate:     5,
			RefillInterval: 30 * time.Second,
		},
		Command: Config{
			MaxRequests:    10,
			WindowDuration: 5 * time.Minute,
			RefillRate:     1,
			RefillInterval: 30 * time.Second,
		},
	}
}

// GetLiberalLimits возвращает либеральные лимиты (больше удобства)
func GetLiberalLimits() ActionLimits {
	return ActionLimits{
		Message: Config{
			MaxRequests:    150,
			WindowDuration: 20 * time.Minute,
			RefillRate:     5,
			RefillInterval: 12 * time.Second,
		},
		FileUpload: Config{
			MaxRequests:    30,
			WindowDuration: 30 * time.Minute,
			RefillRate:     2,
			RefillInterval: 60 * time.Second,
		},
		Callback: Config{
			MaxRequests:    300,
			WindowDuration: 20 * time.Minute,
			RefillRate:     15,
			RefillInterval: 8 * time.Second,
		},
		Command: Config{
			MaxRequests:    30,
			WindowDuration: 15 * time.Minute,
			RefillRate:     2,
			RefillInterval: 20 * time.Second,
		},
	}
}
