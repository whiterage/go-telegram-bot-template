package router

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"strings"
	"time"

	"tgbot/internal/constants"
	"tgbot/internal/logger"
	"tgbot/internal/storage"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/jung-kurt/gofpdf"
)

// handleAnalyticsCommand обрабатывает команду /analytics
func (r *Router) handleAnalyticsCommand(msg *tgbotapi.Message) {
	// Проверяем права доступа
	if !r.isAdmin(msg.From.ID) {
		return
	}

	// Парсим аргументы команды
	args := strings.Fields(msg.Text)
	period := "week" // по умолчанию неделя

	if len(args) > 1 {
		period = strings.ToLower(args[1])
	}

	var stats *storage.WeeklyStats
	var err error
	var periodName string
	var startDate, endDate time.Time

	switch period {
	case "week", "w":
		now := time.Now()
		startDate = getWeekStart(now)
		endDate = startDate.AddDate(0, 0, 7)
		stats, err = r.store.GetWeeklyStats(startDate.Unix(), endDate.Unix())
		periodName = "неделю"

	case "month", "m":
		now := time.Now()
		startDate = time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
		endDate = startDate.AddDate(0, 1, 0)
		stats, err = r.store.GetMonthlyStats(startDate.Unix(), endDate.Unix())
		periodName = "месяц"

	case "year", "y":
		now := time.Now()
		startDate = time.Date(now.Year(), 1, 1, 0, 0, 0, 0, now.Location())
		endDate = time.Date(now.Year()+1, 1, 1, 0, 0, 0, 0, now.Location())
		stats, err = r.store.GetYearlyStats(startDate.Unix(), endDate.Unix())
		periodName = "год"

	case "total", "all", "t":
		stats, err = r.store.GetTotalStats()
		periodName = "все время"

	default:
		reply := tgbotapi.NewMessage(msg.Chat.ID, `❌ Неверный период. Доступные варианты:
📅 /analytics week - текущая неделя
📅 /analytics month - текущий месяц  
📅 /analytics year - текущий год
📅 /analytics total - общая статистика`)
		if _, err := r.bot.Send(reply); err != nil {
			logger.LogSendError(err, msg.Chat.ID, "analytics help")
		}
		return
	}

	if err != nil {
		logger.LogOrderError(err, 0, "get stats")
		reply := tgbotapi.NewMessage(msg.Chat.ID, "❌ Ошибка при получении статистики")
		if _, err := r.bot.Send(reply); err != nil {
			logger.LogSendError(err, msg.Chat.ID, "analytics error")
		}
		return
	}

	// Формируем отчет
	report := r.buildAnalyticsReport(stats, periodName, startDate, endDate)

	reply := tgbotapi.NewMessage(msg.Chat.ID, report)
	if _, err := r.bot.Send(reply); err != nil {
		logger.LogSendError(err, msg.Chat.ID, "analytics report")
	}
}

// buildAnalyticsReport формирует отчет для любого периода
func (r *Router) buildAnalyticsReport(stats *storage.WeeklyStats, periodName string, startDate, endDate time.Time) string {
	var periodInfo string

	if periodName == "все время" {
		periodInfo = "📅 Период: За все время"
	} else {
		periodInfo = fmt.Sprintf("📅 Период: %s - %s",
			startDate.Format("02.01.2006"),
			endDate.AddDate(0, 0, -1).Format("02.01.2006"))
	}

	report := fmt.Sprintf(`📊 **Аналитика за %s**
%s

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
		periodName,
		periodInfo,
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

// getWeekStart возвращает начало недели (понедельник)
func getWeekStart(t time.Time) time.Time {
	weekday := int(t.Weekday())
	if weekday == 0 {
		weekday = 7 // Воскресенье = 7
	}
	return t.AddDate(0, 0, -(weekday - 1)).Truncate(24 * time.Hour)
}

func (r *Router) handleExportCommand(msg *tgbotapi.Message) {
	if !r.isAdmin(msg.From.ID) {
		return
	}

	args := strings.Fields(msg.Text)
	if len(args) < 2 {
		r.replyText(msg.Chat.ID, "Использование: /export month|year|total")
		return
	}
	mode := strings.ToLower(args[1])

	var start, end time.Time
	now := time.Now()

	switch mode {
	case "month", "m":
		start = time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
		end = start.AddDate(0, 1, 0)
	case "year", "y":
		start = time.Date(now.Year(), 1, 1, 0, 0, 0, 0, now.Location())
		end = time.Date(now.Year()+1, 1, 1, 0, 0, 0, 0, now.Location())
	case "total", "all", "t":
		start = time.Unix(0, 0)
		end = now.AddDate(100, 0, 0) // «до бесконечности»
	default:
		r.replyText(msg.Chat.ID, "Неверный период. Используйте: /export month|year|total")
		return
	}

	// Вытащим заявки за период
	orders, err := r.store.GetOrdersRange(start.Unix(), end.Unix(), 5000)
	if err != nil || len(orders) == 0 {
		r.replyText(msg.Chat.ID, "Нет данных для экспорта.")
		return
	}

	// CSV
	csvBytes, csvName, err := buildCSV(orders, mode, start, end)
	if err != nil {
		r.replyText(msg.Chat.ID, "Ошибка формирования CSV")
		return
	}
	// PDF
	pdfBytes, pdfName, err := buildPDF(orders, mode, start, end)
	if err != nil {
		r.replyText(msg.Chat.ID, "Ошибка формирования PDF")
		return
	}

	docCSV := tgbotapi.FileBytes{Name: csvName, Bytes: csvBytes}
	docPDF := tgbotapi.FileBytes{Name: pdfName, Bytes: pdfBytes}

	// Отправляем 2 документа подряд
	if _, err := r.bot.Send(tgbotapi.NewDocument(msg.Chat.ID, docCSV)); err != nil {
		logger.LogSendError(err, msg.Chat.ID, "export_csv_send")
	}
	if _, err := r.bot.Send(tgbotapi.NewDocument(msg.Chat.ID, docPDF)); err != nil {
		logger.LogSendError(err, msg.Chat.ID, "export_pdf_send")
	}
}

func buildCSV(list []storage.Order, mode string, start, end time.Time) ([]byte, string, error) {
	var buf bytes.Buffer
	w := csv.NewWriter(&buf)
	_ = w.Write([]string{"ID", "CreatedAt", "UserID", "Service", "Deadline", "Pages", "Status", "PaymentAmount", "PaymentDate"})
	for _, o := range list {
		created := time.Unix(o.CreatedAt, 0).Format("2006-01-02 15:04")
		payDate := ""
		if o.PaymentDate > 0 {
			payDate = time.Unix(o.PaymentDate, 0).Format("2006-01-02 15:04")
		}
		_ = w.Write([]string{
			fmt.Sprintf("%d", o.ID),
			created,
			fmt.Sprintf("%d", o.UserID),
			o.Service,
			o.DeadlineRaw,
			o.Pages,
			constants.HumanStatus(o.Status),
			fmt.Sprintf("%.2f", o.PaymentAmount),
			payDate,
		})
	}
	w.Flush()
	if err := w.Error(); err != nil {
		return nil, "", err
	}

	name := fmt.Sprintf("export_%s_%s.csv", mode, time.Now().Format("20060102_150405"))
	return buf.Bytes(), name, nil
}

func buildPDF(list []storage.Order, mode string, start, end time.Time) ([]byte, string, error) {
	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.SetFont("Arial", "", 10)
	pdf.AddPage()

	title := fmt.Sprintf("Экспорт заявок — %s (%s–%s)", mode, start.Format("02.01.2006"), end.AddDate(0, 0, -1).Format("02.01.2006"))
	pdf.SetFont("Arial", "B", 12)
	pdf.Cell(0, 8, title)
	pdf.Ln(10)
	pdf.SetFont("Arial", "", 9)

	// Заголовки колонок
	headers := []string{"ID", "Создано", "Польз.", "Работа", "Дедлайн", "Статус", "Сумма"}
	widths := []float64{12, 26, 18, 28, 24, 24, 18}

	for i, h := range headers {
		pdf.CellFormat(widths[i], 7, h, "1", 0, "L", false, 0, "")
	}
	pdf.Ln(-1)

	// Строки
	for _, o := range list {
		created := time.Unix(o.CreatedAt, 0).Format("02.01 15:04")
		row := []string{
			fmt.Sprintf("%d", o.ID),
			created,
			fmt.Sprintf("%d", o.UserID),
			trunc(o.Service, 20),
			trunc(o.DeadlineRaw, 12),
			trunc(constants.HumanStatus(o.Status), 12),
			fmt.Sprintf("%.0f", o.PaymentAmount),
		}
		for i, cell := range row {
			pdf.CellFormat(widths[i], 6, cell, "1", 0, "L", false, 0, "")
		}
		pdf.Ln(-1)
	}

	var out bytes.Buffer
	if err := pdf.Output(&out); err != nil {
		return nil, "", err
	}
	name := fmt.Sprintf("export_%s_%s.pdf", mode, time.Now().Format("20060102_150405"))
	return out.Bytes(), name, nil
}

func trunc(s string, n int) string {
	rs := []rune(s)
	if len(rs) <= n {
		return s
	}
	return string(rs[:n-1]) + "…"
}
