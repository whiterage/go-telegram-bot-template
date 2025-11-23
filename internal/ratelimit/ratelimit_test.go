package ratelimit

import (
	"testing"
	"time"
)

func TestRateLimiter(t *testing.T) {
	// Создаем лимитер с очень строгими лимитами для тестирования
	config := Config{
		MaxRequests:    3,                // только 3 запроса
		WindowDuration: 1 * time.Minute,  // за минуту
		RefillRate:     1,                // 1 токен в минуту
		RefillInterval: 30 * time.Second, // каждые 30 секунд
	}

	rl := NewRateLimiter(config)
	userID := int64(12345)

	// Первые 3 запроса должны проходить
	for i := 0; i < 3; i++ {
		if !rl.IsAllowed(userID) {
			t.Errorf("Запрос %d должен был пройти", i+1)
		}
	}

	// 4-й запрос должен быть заблокирован
	if rl.IsAllowed(userID) {
		t.Error("4-й запрос должен был быть заблокирован")
	}

	// Проверяем статистику
	stats := rl.GetStats()
	if stats["active_users"] != 1 {
		t.Errorf("Ожидалось 1 активный пользователь, получено %v", stats["active_users"])
	}
}

func TestManager(t *testing.T) {
	// Создаем менеджер с тестовыми лимитами
	limits := ActionLimits{
		Message: Config{
			MaxRequests:    2,
			WindowDuration: 1 * time.Minute,
			RefillRate:     1,
			RefillInterval: 30 * time.Second,
		},
		Callback: Config{
			MaxRequests:    5,
			WindowDuration: 1 * time.Minute,
			RefillRate:     2,
			RefillInterval: 30 * time.Second,
		},
	}

	manager := NewManager(limits)
	userID := int64(54321)

	// Тестируем лимиты для сообщений
	for i := 0; i < 2; i++ {
		if !manager.IsAllowed(userID, ActionMessage) {
			t.Errorf("Сообщение %d должно было пройти", i+1)
		}
	}

	// 3-е сообщение должно быть заблокировано
	if manager.IsAllowed(userID, ActionMessage) {
		t.Error("3-е сообщение должно было быть заблокировано")
	}

	// Но callback'и должны проходить (другие лимиты)
	for i := 0; i < 5; i++ {
		if !manager.IsAllowed(userID, ActionCallback) {
			t.Errorf("Callback %d должен был пройти", i+1)
		}
	}

	// 6-й callback должен быть заблокирован
	if manager.IsAllowed(userID, ActionCallback) {
		t.Error("6-й callback должен был быть заблокирован")
	}
}

func TestRefill(t *testing.T) {
	// Создаем лимитер с быстрым восстановлением
	config := Config{
		MaxRequests:    2,
		WindowDuration: 1 * time.Minute,
		RefillRate:     1,
		RefillInterval: 100 * time.Millisecond, // очень быстро
	}

	rl := NewRateLimiter(config)
	userID := int64(99999)

	// Исчерпываем лимит
	rl.IsAllowed(userID)
	rl.IsAllowed(userID)

	// 3-й запрос должен быть заблокирован
	if rl.IsAllowed(userID) {
		t.Error("3-й запрос должен был быть заблокирован")
	}

	// Ждем восстановления токенов
	time.Sleep(150 * time.Millisecond)

	// Теперь запрос должен пройти
	if !rl.IsAllowed(userID) {
		t.Error("Запрос после восстановления должен был пройти")
	}
}
