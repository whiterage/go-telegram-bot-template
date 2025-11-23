package scheduler

import (
	"context"
	"log"
	"time"

	router "tgbot/internal/handlers"
)

// Scheduler управляет периодическими задачами
type Scheduler struct {
	router *router.Router
	ticker *time.Ticker
	ctx    context.Context
	cancel context.CancelFunc
}

// NewScheduler создает новый планировщик
func NewScheduler(router *router.Router) *Scheduler {
	ctx, cancel := context.WithCancel(context.Background())
	return &Scheduler{
		router: router,
		ctx:    ctx,
		cancel: cancel,
	}
}

// Start запускает планировщик
func (s *Scheduler) Start() {
	log.Println("Starting scheduler...")

	// Проверка дедлайнов каждые 6 часов
	s.ticker = time.NewTicker(6 * time.Hour)

	go s.runDeadlineChecker()

	// Еженедельные отчеты по понедельникам в 9:00
	go s.runWeeklyReports()

	log.Println("Scheduler started")
}

// Stop останавливает планировщик
func (s *Scheduler) Stop() {
	log.Println("Stopping scheduler...")
	s.cancel()
	if s.ticker != nil {
		s.ticker.Stop()
	}
	log.Println("Scheduler stopped")
}

// runDeadlineChecker запускает проверку дедлайнов
func (s *Scheduler) runDeadlineChecker() {
	// Первая проверка сразу при запуске
	s.router.CheckDeadlines()

	for {
		select {
		case <-s.ticker.C:
			s.router.CheckDeadlines()
		case <-s.ctx.Done():
			return
		}
	}
}

// runWeeklyReports запускает еженедельные отчеты
func (s *Scheduler) runWeeklyReports() {
	// Вычисляем время до следующего понедельника 9:00
	now := time.Now()
	nextMonday := getNextMonday(now)
	nextReport := time.Date(nextMonday.Year(), nextMonday.Month(), nextMonday.Day(), 9, 0, 0, 0, now.Location())

	// Если уже прошло время сегодняшнего отчета, ждем следующего понедельника
	if nextReport.Before(now) {
		nextReport = nextReport.AddDate(0, 0, 7)
	}

	log.Printf("Next weekly report scheduled for: %s", nextReport.Format("2006-01-02 15:04:05"))

	// Ждем до времени отчета
	timer := time.NewTimer(time.Until(nextReport))
	defer timer.Stop()

	select {
	case <-timer.C:
		s.router.SendWeeklyReport()

		// Планируем следующий отчет через неделю
		go s.runWeeklyReports()
	case <-s.ctx.Done():
		return
	}
}

// getNextMonday возвращает следующий понедельник
func getNextMonday(t time.Time) time.Time {
	weekday := int(t.Weekday())
	if weekday == 0 {
		weekday = 7 // Воскресенье = 7
	}
	daysUntilMonday := (8 - weekday) % 7
	if daysUntilMonday == 0 {
		daysUntilMonday = 7
	}
	return t.AddDate(0, 0, daysUntilMonday)
}
