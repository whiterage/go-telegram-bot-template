package ratelimit

import (
	"sync"
	"time"
)

// Config содержит настройки rate limiting
type Config struct {
	MaxRequests    int           // максимальное количество запросов
	WindowDuration time.Duration // окно времени
	RefillRate     int           // скорость восстановления токенов
	RefillInterval time.Duration // интервал восстановления
}

// UserLimiter отслеживает лимиты для одного пользователя
type UserLimiter struct {
	tokens     int
	lastRefill time.Time
	requests   []time.Time // для sliding window
}

// RateLimiter управляет лимитами для всех пользователей
type RateLimiter struct {
	mu      sync.RWMutex
	users   map[int64]*UserLimiter
	config  Config
	cleanup *time.Timer
}

// NewRateLimiter создает новый rate limiter
func NewRateLimiter(config Config) *RateLimiter {
	rl := &RateLimiter{
		users:  make(map[int64]*UserLimiter),
		config: config,
	}

	// Запускаем автоочистку неактивных пользователей
	rl.startCleanup()
	return rl
}

// IsAllowed проверяет, разрешен ли запрос для пользователя
func (rl *RateLimiter) IsAllowed(userID int64) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()

	// Получаем или создаем лимитер для пользователя
	limiter, exists := rl.users[userID]
	if !exists {
		limiter = &UserLimiter{
			tokens:     rl.config.MaxRequests,
			lastRefill: now,
		}
		rl.users[userID] = limiter
	}

	// Восстанавливаем токены
	timePassed := now.Sub(limiter.lastRefill)
	tokensToAdd := int(timePassed/rl.config.RefillInterval) * rl.config.RefillRate
	if tokensToAdd > 0 {
		limiter.tokens = min(limiter.tokens+tokensToAdd, rl.config.MaxRequests)
		limiter.lastRefill = now
	}

	// Проверяем лимит
	if limiter.tokens > 0 {
		limiter.tokens--
		return true
	}

	return false
}

// startCleanup запускает периодическую очистку неактивных пользователей
func (rl *RateLimiter) startCleanup() {
	go func() {
		ticker := time.NewTicker(5 * time.Minute)
		for range ticker.C {
			rl.cleanupInactiveUsers()
		}
	}()
}

// cleanupInactiveUsers удаляет неактивных пользователей
func (rl *RateLimiter) cleanupInactiveUsers() {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	cutoff := time.Now().Add(-10 * time.Minute)
	for userID, limiter := range rl.users {
		if limiter.lastRefill.Before(cutoff) {
			delete(rl.users, userID)
		}
	}
}

// GetStats возвращает статистику rate limiter
func (rl *RateLimiter) GetStats() map[string]interface{} {
	rl.mu.RLock()
	defer rl.mu.RUnlock()

	return map[string]interface{}{
		"active_users": len(rl.users),
		"config":       rl.config,
	}
}

// min возвращает минимальное из двух чисел
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
