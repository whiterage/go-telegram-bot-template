package scheduler

import (
	"log"
	"time"

	"tgbot/internal/reports"
)

// WeeklyScheduler управляет еженедельными отчетами
type WeeklyScheduler struct {
	reporter *reports.WeeklyReporter
	ticker   *time.Ticker
	done     chan bool
}

// NewWeeklyScheduler создает новый планировщик еженедельных отчетов
func NewWeeklyScheduler(reporter *reports.WeeklyReporter) *WeeklyScheduler {
	return &WeeklyScheduler{
		reporter: reporter,
		done:     make(chan bool),
	}
}

// Start запускает планировщик
func (ws *WeeklyScheduler) Start() {
	// Вычисляем время до следующего понедельника
	now := time.Now()
	daysUntilMonday := (8 - int(now.Weekday())) % 7
	if daysUntilMonday == 0 {
		daysUntilMonday = 7
	}

	nextMonday := now.AddDate(0, 0, daysUntilMonday)
	nextMonday = time.Date(nextMonday.Year(), nextMonday.Month(), nextMonday.Day(), 9, 0, 0, 0, nextMonday.Location())

	// Если уже понедельник и время больше 9:00, планируем на следующий понедельник
	if now.Weekday() == time.Monday && now.Hour() >= 9 {
		nextMonday = nextMonday.AddDate(0, 0, 7)
	}

	delay := time.Until(nextMonday)
	log.Printf("Weekly report scheduled for: %s (in %v)", nextMonday.Format("2006-01-02 15:04"), delay)

	// Запускаем тикер каждую неделю
	ws.ticker = time.NewTicker(7 * 24 * time.Hour)

	go func() {
		// Ждем до первого понедельника
		time.Sleep(delay)

		// Отправляем первый отчет
		ws.sendWeeklyReport()

		// Затем каждую неделю
		for {
			select {
			case <-ws.ticker.C:
				ws.sendWeeklyReport()
			case <-ws.done:
				return
			}
		}
	}()
}

// Stop останавливает планировщик
func (ws *WeeklyScheduler) Stop() {
	log.Println("Stopping weekly scheduler...")
	if ws.ticker != nil {
		ws.ticker.Stop()
	}
	select {
	case ws.done <- true:
	default:
		// Канал уже закрыт или заблокирован
	}
	log.Println("Weekly scheduler stopped")
}

// sendWeeklyReport отправляет еженедельный отчет
func (ws *WeeklyScheduler) sendWeeklyReport() {
	log.Println("Generating and sending weekly report...")

	if err := ws.reporter.SendWeeklyReport(); err != nil {
		log.Printf("Failed to send weekly report: %v", err)
	} else {
		log.Println("Weekly report sent successfully")
	}
}

// SendReportNow отправляет отчет немедленно (для тестирования)
func (ws *WeeklyScheduler) SendReportNow() error {
	log.Println("Sending weekly report now...")
	return ws.reporter.SendWeeklyReport()
}
