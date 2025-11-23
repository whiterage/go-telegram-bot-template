package router

import (
	"fmt"
	"html"
	"strings"
	"time"

	"tgbot/internal/logger"
	"tgbot/internal/parsing"
	"tgbot/internal/storage"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// DeadlineNotification содержит информацию об уведомлении о дедлайне
type DeadlineNotification struct {
	OrderID  int64
	UserID   int64
	Deadline time.Time
	Service  string
	DaysLeft int
}

// CheckDeadlines проверяет дедлайны и отправляет уведомления
func (r *Router) CheckDeadlines() {
	// Получаем заявки с приближающимися дедлайнами
	orders, err := r.store.GetOrdersByDeadline(7) // за 7 дней
	if err != nil {
		logger.LogOrderError(err, 0, "get orders by deadline")
		return
	}

	var notifications []DeadlineNotification

	for _, order := range orders {
		// Парсим дедлайн
		result := parsing.ParseDeadline(order.DeadlineRaw)
		if !result.IsValid || result.ParsedDate == nil {
			continue // Пропускаем заявки с неизвестными дедлайнами
		}
		deadline := *result.ParsedDate

		// Проверяем, сколько дней осталось
		now := time.Now().In(deadline.Location())
		today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, deadline.Location())
		dday := time.Date(deadline.Year(), deadline.Month(), deadline.Day(), 0, 0, 0, 0, deadline.Location())
		daysLeft := int(dday.Sub(today) / (24 * time.Hour))

		// Отправляем уведомления за 3, 1 день и в день дедлайна
		if daysLeft == 3 || daysLeft == 1 || daysLeft == 0 {
			notifications = append(notifications, DeadlineNotification{
				OrderID:  order.ID,
				UserID:   order.UserID,
				Deadline: deadline,
				Service:  order.Service,
				DaysLeft: daysLeft,
			})
		}
	}

	if len(notifications) == 0 {
		return
	}

	// Отправляем уведомления
	r.sendDeadlineNotifications(notifications)
}

// sendDeadlineNotifications отправляет уведомления о дедлайнах
func (r *Router) sendDeadlineNotifications(notifications []DeadlineNotification) {
	// Группируем уведомления по дням
	grouped := make(map[int][]DeadlineNotification)
	for _, notif := range notifications {
		grouped[notif.DaysLeft] = append(grouped[notif.DaysLeft], notif)
	}

	// Отправляем уведомления для каждого дня
	for daysLeft, notifs := range grouped {
		message := r.buildDeadlineMessage(daysLeft, notifs) // HTML
		// единое сообщение в тему дедлайнов (forum topic)
		if _, err := r.sendMessageToThread(r.boardChatID, r.deadlineTopicID, message, nil); err != nil {
			// если по каким-то причинам тема недоступна — фолбэк: рассылаем в личку админам
			for adminID := range r.adminIDs {
				m := tgbotapi.NewMessage(adminID, stripHtml(message))
				if _, e := r.bot.Send(m); e != nil {
					logger.LogSendError(e, adminID, "deadline notification fallback")
				}
			}
		}
	}
}

// buildDeadlineMessage формирует сообщение об уведомлении
func (r *Router) buildDeadlineMessage(daysLeft int, notifications []DeadlineNotification) string {
	var header string
	var emoji string

	switch daysLeft {
	case 0:
		header = "🚨 <b>СРОЧНО! Дедлайны сегодня:</b>"
		emoji = "🚨"
	case 1:
		header = "⚠️ <b>Дедлайны завтра:</b>"
		emoji = "⚠️"
	case 3:
		header = "📅 <b>Дедлайны через 3 дня:</b>"
		emoji = "📅"
	default:
		header = fmt.Sprintf("📅 <b>Дедлайны через %d дней:</b>", daysLeft)
		emoji = "📅"
	}

	var items []string
	for _, notif := range notifications {
		item := fmt.Sprintf("%s Заявка №%d: %s",
			emoji, notif.OrderID, html.EscapeString(notif.Service))
		items = append(items, item)
	}

	return header + "\n\n" + strings.Join(items, "\n")
}

func stripHtml(s string) string {
	// минимализм: убираем только <b> и </b>; остальное у нас не используется
	s = strings.ReplaceAll(s, "<b>", "")
	s = strings.ReplaceAll(s, "</b>", "")
	return s
}

// SendWeeklyReport отправляет еженедельный отчет
func (r *Router) SendWeeklyReport() {
	// Получаем статистику за прошлую неделю
	now := time.Now()
	lastWeekStart := getWeekStart(now).AddDate(0, 0, -7)
	lastWeekEnd := lastWeekStart.AddDate(0, 0, 7).Unix()

	stats, err := r.store.GetWeeklyStats(lastWeekStart.Unix(), lastWeekEnd)
	if err != nil {
		logger.LogOrderError(err, 0, "get weekly stats for report")
		return
	}

	// Формируем отчет
	report := r.buildWeeklyReport(stats, lastWeekStart)

	// Отправляем отчет всем админам
	for adminID := range r.adminIDs {
		msg := tgbotapi.NewMessage(adminID, report)
		if _, err := r.bot.Send(msg); err != nil {
			logger.LogSendError(err, adminID, "weekly report")
		}
	}
}

// buildWeeklyReport формирует еженедельный отчет
func (r *Router) buildWeeklyReport(stats *storage.WeeklyStats, weekStart time.Time) string {
	weekEnd := weekStart.AddDate(0, 0, 6)

	report := fmt.Sprintf(`📊 **Еженедельный отчет**
📅 Период: %s - %s

📈 **Статистика заявок:**
• Всего заявок: %d
• Оплачено: %d
• В ожидании: %d
• Отклонено: %d

💰 **Финансы:**
• Выручка: %s руб.
• Возвраты: %s руб.
• Чистая прибыль: %s руб.

📊 **Конверсия:**
• Успешность: %.1f%%
• Отклонения: %.1f%%`,
		weekStart.Format("02.01.2006"),
		weekEnd.Format("02.01.2006"),
		stats.TotalOrders,
		stats.PaidOrders,
		stats.PendingOrders,
		stats.RejectedOrders,
		formatAmount(stats.TotalRevenue),
		formatAmount(stats.TotalRefunds),
		formatAmount(stats.TotalRevenue-stats.TotalRefunds),
		r.calculateSuccessRate(stats),
		r.calculateRejectionRate(stats))

	return report
}

// calculateSuccessRate рассчитывает процент успешных заявок
func (r *Router) calculateSuccessRate(stats *storage.WeeklyStats) float64 {
	if stats.TotalOrders == 0 {
		return 0
	}
	return float64(stats.PaidOrders) / float64(stats.TotalOrders) * 100
}

// calculateRejectionRate рассчитывает процент отклоненных заявок
func (r *Router) calculateRejectionRate(stats *storage.WeeklyStats) float64 {
	if stats.TotalOrders == 0 {
		return 0
	}
	return float64(stats.RejectedOrders) / float64(stats.TotalOrders) * 100
}
