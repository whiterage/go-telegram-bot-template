package scheduler

import (
	"log"
	"sync"
	"time"

	"tgbot/internal/reports"
)

// reportWeekday и reportHour задают расписание: понедельник, 09:00 локального времени.
const (
	reportWeekday = time.Monday
	reportHour    = 9
)

// WeeklyScheduler управляет еженедельными отчетами
type WeeklyScheduler struct {
	reporter *reports.WeeklyReporter
	done     chan struct{}
	stopOnce sync.Once
}

// NewWeeklyScheduler создает новый планировщик еженедельных отчетов
func NewWeeklyScheduler(reporter *reports.WeeklyReporter) *WeeklyScheduler {
	return &WeeklyScheduler{
		reporter: reporter,
		done:     make(chan struct{}),
	}
}

// nextReportTime возвращает ближайший понедельник 09:00 строго после from.
func nextReportTime(from time.Time) time.Time {
	daysAhead := (int(reportWeekday) - int(from.Weekday()) + 7) % 7
	candidate := time.Date(from.Year(), from.Month(), from.Day(), reportHour, 0, 0, 0, from.Location()).
		AddDate(0, 0, daysAhead)

	// Сегодня понедельник, но 09:00 уже прошло — значит следующий понедельник.
	if !candidate.After(from) {
		candidate = candidate.AddDate(0, 0, 7)
	}
	return candidate
}

// Start запускает планировщик
func (ws *WeeklyScheduler) Start() {
	next := nextReportTime(time.Now())
	log.Printf("Weekly report scheduled for: %s (in %v)", next.Format("2006-01-02 15:04"), time.Until(next))

	go func() {
		for {
			timer := time.NewTimer(time.Until(next))
			select {
			case <-timer.C:
				ws.sendWeeklyReport()
				// Пересчитываем от текущего момента, а не прибавляем ровно 7*24ч:
				// так расписание переживает переход на летнее/зимнее время
				// и не уплывает от 09:00 понедельника.
				next = nextReportTime(time.Now())
				log.Printf("Next weekly report: %s", next.Format("2006-01-02 15:04"))
			case <-ws.done:
				timer.Stop()
				return
			}
		}
	}()
}

// Stop останавливает планировщик
func (ws *WeeklyScheduler) Stop() {
	ws.stopOnce.Do(func() {
		log.Println("Stopping weekly scheduler...")
		close(ws.done)
		log.Println("Weekly scheduler stopped")
	})
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
