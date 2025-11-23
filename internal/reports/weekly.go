package reports

import (
	"fmt"
	"strings"
	"time"

	"tgbot/internal/health"
	"tgbot/internal/storage"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// WeeklyReporter создает еженедельные отчеты
type WeeklyReporter struct {
	bot       *tgbotapi.BotAPI
	store     *storage.Store
	health    *health.HealthChecker
	adminIDs  []int64
	channelID int64
}

// NewWeeklyReporter создает новый weekly reporter
func NewWeeklyReporter(bot *tgbotapi.BotAPI, store *storage.Store, health *health.HealthChecker, adminIDs []int64, channelID int64) *WeeklyReporter {
	return &WeeklyReporter{
		bot:       bot,
		store:     store,
		health:    health,
		adminIDs:  adminIDs,
		channelID: channelID,
	}
}

// GenerateWeeklyReport создает еженедельный отчет
func (wr *WeeklyReporter) GenerateWeeklyReport() (string, error) {
	now := time.Now()
	weekStart := now.AddDate(0, 0, -7)
	monthStart := now.AddDate(0, 0, -30)

	// Получаем метрики
	totalUsers, _ := wr.store.GetUsersCount()
	totalOrders, _ := wr.store.GetOrdersCount()
	ordersThisWeek, _ := wr.store.GetOrdersCountSince(weekStart)
	ordersThisMonth, _ := wr.store.GetOrdersCountSince(monthStart)

	// Получаем статистику по статусам за эту неделю
	statusStats, _ := wr.getStatusStats(weekStart, now)

	// Получаем топ-5 заявок по объему
	topOrders, _ := wr.getTopOrders(weekStart, now)

	// Получаем статистику по типам работ
	workTypeStats, _ := wr.getWorkTypeStats(weekStart, now)

	// Получаем статистику по заведениям
	institutionStats, _ := wr.getInstitutionStats(weekStart, now)

	var report strings.Builder

	// Заголовок
	report.WriteString("📊 **ЕЖЕНЕДЕЛЬНЫЙ ОТЧЕТ БОТА**\n")
	report.WriteString(fmt.Sprintf("📅 Период: %s - %s\n\n",
		weekStart.Format("02.01.2006"), now.Format("02.01.2006")))

	// Общая статистика
	report.WriteString("📈 **ОБЩАЯ СТАТИСТИКА**\n")
	report.WriteString(fmt.Sprintf("👥 Всего пользователей: %d\n", totalUsers))
	report.WriteString(fmt.Sprintf("📝 Всего заявок: %d\n", totalOrders))
	report.WriteString(fmt.Sprintf("📋 Заявок за неделю: %d\n", ordersThisWeek))
	report.WriteString(fmt.Sprintf("📋 Заявок за месяц: %d\n\n", ordersThisMonth))

	// Статистика по статусам
	if len(statusStats) > 0 {
		report.WriteString("📊 **СТАТИСТИКА ПО СТАТУСАМ (неделя)**\n")
		for status, count := range statusStats {
			report.WriteString(fmt.Sprintf("• %s: %d\n", status, count))
		}
		report.WriteString("\n")
	}

	// Статистика по типам работ
	if len(workTypeStats) > 0 {
		report.WriteString("🔧 **ПОПУЛЯРНЫЕ ТИПЫ РАБОТ (неделя)**\n")
		for i, stat := range workTypeStats {
			if i >= 5 { // Показываем только топ-5
				break
			}
			report.WriteString(fmt.Sprintf("%d. %s: %d заявок\n", i+1, stat.WorkType, stat.Count))
		}
		report.WriteString("\n")
	}

	// Статистика по заведениям
	if len(institutionStats) > 0 {
		report.WriteString("🏫 **ПОПУЛЯРНЫЕ ЗАВЕДЕНИЯ (неделя)**\n")
		for i, stat := range institutionStats {
			if i >= 5 { // Показываем только топ-5
				break
			}
			report.WriteString(fmt.Sprintf("%d. %s: %d заявок\n", i+1, stat.Institution, stat.Count))
		}
		report.WriteString("\n")
	}

	// Топ заявки по объему
	if len(topOrders) > 0 {
		report.WriteString("💰 **ТОП ЗАЯВКИ ПО ОБЪЕМУ (неделя)**\n")
		for i, order := range topOrders {
			if i >= 3 { // Показываем только топ-3
				break
			}
			report.WriteString(fmt.Sprintf("%d. ID %d: %s страниц - %s\n",
				i+1, order.ID, order.Pages, order.Service))
		}
		report.WriteString("\n")
	}

	// Заключение
	report.WriteString("📈 **ВЫВОДЫ**\n")
	if ordersThisWeek > 0 {
		avgOrdersPerDay := float64(ordersThisWeek) / 7
		report.WriteString(fmt.Sprintf("• Среднедневная активность: %.1f заявок/день\n", avgOrdersPerDay))
	}

	if ordersThisWeek > ordersThisMonth/4 {
		report.WriteString("• 📈 Рост активности по сравнению с предыдущими неделями\n")
	} else if ordersThisWeek < ordersThisMonth/4 {
		report.WriteString("• 📉 Снижение активности по сравнению с предыдущими неделями\n")
	} else {
		report.WriteString("• 📊 Стабильная активность\n")
	}

	report.WriteString("\n🤖 Бот работает стабильно\n")
	report.WriteString(fmt.Sprintf("⏰ Отчет сгенерирован: %s", now.Format("02.01.2006 15:04")))

	return report.String(), nil
}

// SendWeeklyReport отправляет еженедельный отчет
func (wr *WeeklyReporter) SendWeeklyReport() error {
	report, err := wr.GenerateWeeklyReport()
	if err != nil {
		return fmt.Errorf("failed to generate weekly report: %w", err)
	}

	// Отправляем админам
	for _, adminID := range wr.adminIDs {
		msg := tgbotapi.NewMessage(adminID, report)
		msg.ParseMode = "Markdown"
		wr.bot.Send(msg)
	}

	// Отправляем в канал (если указан)
	if wr.channelID != 0 {
		msg := tgbotapi.NewMessage(wr.channelID, report)
		msg.ParseMode = "Markdown"
		wr.bot.Send(msg)
	}

	return nil
}

// StatusStat представляет статистику по статусам
type StatusStat struct {
	Status string
	Count  int64
}

// WorkTypeStat представляет статистику по типам работ
type WorkTypeStat struct {
	WorkType string
	Count    int64
}

// InstitutionStat представляет статистику по заведениям
type InstitutionStat struct {
	Institution string
	Count       int64
}

// getStatusStats получает статистику по статусам
func (wr *WeeklyReporter) getStatusStats(start, end time.Time) (map[string]int64, error) {
	statuses := []string{"pending", "in_progress", "paid", "done"}
	stats := make(map[string]int64)

	for _, status := range statuses {
		orders, err := wr.store.GetOrdersByStatusSince(status, start)
		if err != nil {
			continue
		}
		stats[status] = int64(len(orders))
	}

	return stats, nil
}

// getWorkTypeStats получает статистику по типам работ
func (wr *WeeklyReporter) getWorkTypeStats(start, end time.Time) ([]WorkTypeStat, error) {
	stats, err := wr.store.GetWorkTypeStats(start)
	if err != nil {
		return nil, err
	}

	var result []WorkTypeStat
	for workType, count := range stats {
		result = append(result, WorkTypeStat{
			WorkType: workType,
			Count:    count,
		})
	}

	// Сортируем по количеству
	for i := 0; i < len(result); i++ {
		for j := i + 1; j < len(result); j++ {
			if result[i].Count < result[j].Count {
				result[i], result[j] = result[j], result[i]
			}
		}
	}

	return result, nil
}

// getInstitutionStats получает статистику по заведениям
func (wr *WeeklyReporter) getInstitutionStats(start, end time.Time) ([]InstitutionStat, error) {
	stats, err := wr.store.GetInstitutionStats(start)
	if err != nil {
		return nil, err
	}

	var result []InstitutionStat
	for institution, count := range stats {
		result = append(result, InstitutionStat{
			Institution: institution,
			Count:       count,
		})
	}

	// Сортируем по количеству
	for i := 0; i < len(result); i++ {
		for j := i + 1; j < len(result); j++ {
			if result[i].Count < result[j].Count {
				result[i], result[j] = result[j], result[i]
			}
		}
	}

	return result, nil
}

// getTopOrders получает топ заявки по объему
func (wr *WeeklyReporter) getTopOrders(start, end time.Time) ([]storage.Order, error) {
	return wr.store.GetTopOrdersByPages(start, 5)
}
