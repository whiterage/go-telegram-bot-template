package ratelimit

import (
	"sync"
)

// Manager управляет всеми типами rate limiting
type Manager struct {
	mu       sync.RWMutex
	limiters map[ActionType]*RateLimiter
}

// NewManager создает новый менеджер rate limiting
func NewManager(limits ActionLimits) *Manager {
	manager := &Manager{
		limiters: make(map[ActionType]*RateLimiter),
	}

	// Создаем лимитеры для каждого типа действий
	manager.limiters[ActionMessage] = NewRateLimiter(limits.Message)
	manager.limiters[ActionFileUpload] = NewRateLimiter(limits.FileUpload)
	manager.limiters[ActionCallback] = NewRateLimiter(limits.Callback)
	manager.limiters[ActionCommand] = NewRateLimiter(limits.Command)

	return manager
}

// IsAllowed проверяет, разрешен ли запрос для определенного типа действия
func (m *Manager) IsAllowed(userID int64, actionType ActionType) bool {
	m.mu.RLock()
	limiter, exists := m.limiters[actionType]
	m.mu.RUnlock()

	if !exists {
		return true // Если лимитер не найден, разрешаем запрос
	}

	return limiter.IsAllowed(userID)
}

// GetStats возвращает статистику для всех лимитеров
func (m *Manager) GetStats() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := make(map[string]interface{})

	for actionType, limiter := range m.limiters {
		actionName := getActionTypeName(actionType)
		stats[actionName] = limiter.GetStats()
	}

	return stats
}

// getActionTypeName возвращает строковое название типа действия
func getActionTypeName(actionType ActionType) string {
	switch actionType {
	case ActionMessage:
		return "message"
	case ActionFileUpload:
		return "file_upload"
	case ActionCallback:
		return "callback"
	case ActionCommand:
		return "command"
	default:
		return "unknown"
	}
}
