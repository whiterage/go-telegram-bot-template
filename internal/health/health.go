package health

import (
	"fmt"
	"sync"
	"time"

	"tgbot/internal/storage"
)

// HealthStatus представляет статус здоровья системы
type HealthStatus struct {
	Status     string               `json:"status"`
	Timestamp  time.Time            `json:"timestamp"`
	Version    string               `json:"version"`
	Uptime     time.Duration        `json:"uptime"`
	Components map[string]Component `json:"components"`
	Metrics    Metrics              `json:"metrics"`
}

// Component представляет состояние компонента системы
type Component struct {
	Status    string        `json:"status"`
	Message   string        `json:"message,omitempty"`
	Latency   time.Duration `json:"latency,omitempty"`
	LastCheck time.Time     `json:"last_check"`
}

// Metrics содержит метрики системы
type Metrics struct {
	TotalUsers      int64 `json:"total_users"`
	ActiveUsers     int64 `json:"active_users"`
	TotalOrders     int64 `json:"total_orders"`
	OrdersThisWeek  int64 `json:"orders_this_week"`
	OrdersThisMonth int64 `json:"orders_this_month"`
	RateLimitBlocks int64 `json:"rate_limit_blocks"`
	DatabaseSize    int64 `json:"database_size_bytes"`
}

// HealthChecker управляет проверками здоровья системы
type HealthChecker struct {
	mu          sync.RWMutex
	startTime   time.Time
	version     string
	store       *storage.Store
	rateLimiter interface{} // для получения статистики rate limiting

	// Метрики
	totalUsers      int64
	activeUsers     int64
	rateLimitBlocks int64
}

// NewHealthChecker создает новый health checker
func NewHealthChecker(version string, store *storage.Store, rateLimiter interface{}) *HealthChecker {
	return &HealthChecker{
		startTime:   time.Now(),
		version:     version,
		store:       store,
		rateLimiter: rateLimiter,
	}
}

// CheckHealth выполняет полную проверку здоровья системы
func (hc *HealthChecker) CheckHealth() HealthStatus {
	hc.mu.Lock()
	defer hc.mu.Unlock()

	components := make(map[string]Component)

	// Проверка базы данных
	dbComponent := hc.checkDatabase()
	components["database"] = dbComponent

	// Проверка rate limiting
	rlComponent := hc.checkRateLimiting()
	components["rate_limiting"] = rlComponent

	// Проверка общего состояния
	overallStatus := "healthy"
	for _, comp := range components {
		if comp.Status != "healthy" {
			overallStatus = "degraded"
			break
		}
	}

	// Получение метрик
	metrics := hc.getMetrics()

	return HealthStatus{
		Status:     overallStatus,
		Timestamp:  time.Now(),
		Version:    hc.version,
		Uptime:     time.Since(hc.startTime),
		Components: components,
		Metrics:    metrics,
	}
}

// checkDatabase проверяет состояние базы данных
func (hc *HealthChecker) checkDatabase() Component {
	start := time.Now()

	// Простая проверка подключения к БД
	_, err := hc.store.GetOrdersCount()
	latency := time.Since(start)

	if err != nil {
		return Component{
			Status:    "unhealthy",
			Message:   fmt.Sprintf("Database error: %v", err),
			Latency:   latency,
			LastCheck: time.Now(),
		}
	}

	status := "healthy"
	if latency > 1*time.Second {
		status = "degraded"
	}

	return Component{
		Status:    status,
		Message:   fmt.Sprintf("Database responding in %v", latency),
		Latency:   latency,
		LastCheck: time.Now(),
	}
}

// checkRateLimiting проверяет состояние rate limiting
func (hc *HealthChecker) checkRateLimiting() Component {
	// Проверяем, что rate limiter доступен
	if hc.rateLimiter == nil {
		return Component{
			Status:    "unhealthy",
			Message:   "Rate limiter not initialized",
			LastCheck: time.Now(),
		}
	}

	return Component{
		Status:    "healthy",
		Message:   "Rate limiter operational",
		LastCheck: time.Now(),
	}
}

// getMetrics собирает метрики системы
func (hc *HealthChecker) getMetrics() Metrics {
	// Получаем общее количество пользователей
	totalUsers, _ := hc.store.GetUsersCount()

	// Получаем количество заявок
	totalOrders, _ := hc.store.GetOrdersCount()

	// Заявки за эту неделю
	weekStart := time.Now().AddDate(0, 0, -7)
	ordersThisWeek, _ := hc.store.GetOrdersCountSince(weekStart)

	// Заявки за этот месяц
	monthStart := time.Now().AddDate(0, 0, -30)
	ordersThisMonth, _ := hc.store.GetOrdersCountSince(monthStart)

	// Размер базы данных (приблизительно)
	dbSize := hc.estimateDatabaseSize()

	return Metrics{
		TotalUsers:      totalUsers,
		ActiveUsers:     hc.activeUsers,
		TotalOrders:     totalOrders,
		OrdersThisWeek:  ordersThisWeek,
		OrdersThisMonth: ordersThisMonth,
		RateLimitBlocks: hc.rateLimitBlocks,
		DatabaseSize:    dbSize,
	}
}

// estimateDatabaseSize оценивает размер базы данных
func (hc *HealthChecker) estimateDatabaseSize() int64 {
	// Простая оценка на основе количества записей
	totalOrders, _ := hc.store.GetOrdersCount()
	totalUsers, _ := hc.store.GetUsersCount()

	// Примерная оценка: 1KB на заявку + 0.5KB на пользователя
	return int64(totalOrders*1024 + totalUsers*512)
}

// UpdateMetrics обновляет метрики
func (hc *HealthChecker) UpdateMetrics(activeUsers int64, rateLimitBlocks int64) {
	hc.mu.Lock()
	defer hc.mu.Unlock()

	hc.activeUsers = activeUsers
	hc.rateLimitBlocks += rateLimitBlocks
}
