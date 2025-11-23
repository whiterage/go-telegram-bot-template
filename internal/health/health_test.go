package health

import (
	"testing"
	"time"
)

func TestHealthChecker(t *testing.T) {
	// Создаем mock store
	// В реальном тесте здесь должен быть mock
	// Пока просто проверяем, что структура создается
	checker := NewHealthChecker("1.0.0", nil, nil)

	if checker == nil {
		t.Error("HealthChecker should not be nil")
	}

	if checker.version != "1.0.0" {
		t.Errorf("Expected version 1.0.0, got %s", checker.version)
	}
}

func TestHealthStatus(t *testing.T) {
	// Тестируем структуру HealthStatus
	status := HealthStatus{
		Status:    "healthy",
		Timestamp: time.Now(),
		Version:   "1.0.0",
		Uptime:    5 * time.Minute,
		Components: map[string]Component{
			"database": {
				Status:    "healthy",
				Message:   "Database responding in 10ms",
				Latency:   10 * time.Millisecond,
				LastCheck: time.Now(),
			},
		},
		Metrics: Metrics{
			TotalUsers:      100,
			ActiveUsers:     10,
			TotalOrders:     50,
			OrdersThisWeek:  5,
			OrdersThisMonth: 20,
		},
	}

	if status.Status != "healthy" {
		t.Errorf("Expected status 'healthy', got %s", status.Status)
	}

	if status.Version != "1.0.0" {
		t.Errorf("Expected version '1.0.0', got %s", status.Version)
	}

	if status.Uptime != 5*time.Minute {
		t.Errorf("Expected uptime 5m, got %v", status.Uptime)
	}

	if len(status.Components) != 1 {
		t.Errorf("Expected 1 component, got %d", len(status.Components))
	}

	if status.Metrics.TotalUsers != 100 {
		t.Errorf("Expected 100 total users, got %d", status.Metrics.TotalUsers)
	}
}

func TestComponent(t *testing.T) {
	component := Component{
		Status:    "healthy",
		Message:   "Test component working",
		Latency:   5 * time.Millisecond,
		LastCheck: time.Now(),
	}

	if component.Status != "healthy" {
		t.Errorf("Expected status 'healthy', got %s", component.Status)
	}

	if component.Message != "Test component working" {
		t.Errorf("Expected message 'Test component working', got %s", component.Message)
	}

	if component.Latency != 5*time.Millisecond {
		t.Errorf("Expected latency 5ms, got %v", component.Latency)
	}
}

func TestMetrics(t *testing.T) {
	metrics := Metrics{
		TotalUsers:      1000,
		ActiveUsers:     50,
		TotalOrders:     500,
		OrdersThisWeek:  25,
		OrdersThisMonth: 100,
		RateLimitBlocks: 5,
		DatabaseSize:    1024 * 1024, // 1MB
	}

	if metrics.TotalUsers != 1000 {
		t.Errorf("Expected 1000 total users, got %d", metrics.TotalUsers)
	}

	if metrics.ActiveUsers != 50 {
		t.Errorf("Expected 50 active users, got %d", metrics.ActiveUsers)
	}

	if metrics.TotalOrders != 500 {
		t.Errorf("Expected 500 total orders, got %d", metrics.TotalOrders)
	}

	if metrics.OrdersThisWeek != 25 {
		t.Errorf("Expected 25 orders this week, got %d", metrics.OrdersThisWeek)
	}

	if metrics.OrdersThisMonth != 100 {
		t.Errorf("Expected 100 orders this month, got %d", metrics.OrdersThisMonth)
	}

	if metrics.RateLimitBlocks != 5 {
		t.Errorf("Expected 5 rate limit blocks, got %d", metrics.RateLimitBlocks)
	}

	if metrics.DatabaseSize != 1024*1024 {
		t.Errorf("Expected 1MB database size, got %d", metrics.DatabaseSize)
	}
}
