package main

import (
	"context"
	"log"
	"net/http"
	"os/signal"
	"strconv"
	"strings"
	"syscall"

	"tgbot/internal/config"
	router "tgbot/internal/handlers"
	"tgbot/internal/health"
	"tgbot/internal/reports"
	"tgbot/internal/scheduler"
	"tgbot/internal/state"
	"tgbot/internal/storage"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func maskToken(t string) string {
	if len(t) <= 6 {
		return "***"
	}
	return t[:5] + "***"
}

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("ошибка загрузки конфига: %v", err)
	}
	log.Printf("Env: %s | UseWebhook=%v", cfg.Env, cfg.UseWebhook)

	bot, err := tgbotapi.NewBotAPI(cfg.BotToken)
	if err != nil {
		log.Fatalf("ошибка инициализации бота: %v", err)
	}
	bot.Debug = (cfg.Env == "dev")

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	st, err := storage.Open("data.db")
	if err != nil {
		log.Fatalf("db open error: %v", err)
	}
	defer func() {
		log.Println("Closing database connection...")
		if err := st.DB.Close(); err != nil {
			log.Printf("Error closing database: %v", err)
		}
	}()

	started := state.NewStartTracker()
	sessions := state.NewSessions()
	r := router.NewRouter(
		bot, started, sessions,
		cfg.ChannelID, cfg.ChannelURL, cfg.AllowAdminsBypass,
		cfg.AdminIDsCSV, st,
		cfg.BoardChatID, cfg.TopicInProgressID, cfg.TopicPaidID, cfg.TopicDoneID, cfg.DeadlineTopicID, cfg.FAQURL,
	)

	// Health checker
	healthChecker := health.NewHealthChecker("1.0.0", st, r.GetRateLimiter())

	// Weekly reporter
	adminIDs := []int64{}
	for _, idStr := range strings.Split(cfg.AdminIDsCSV, ",") {
		idStr = strings.TrimSpace(idStr)
		if idStr == "" {
			continue
		}
		if id, err := strconv.ParseInt(idStr, 10, 64); err == nil {
			adminIDs = append(adminIDs, id)
		}
	}
	weeklyReporter := reports.NewWeeklyReporter(bot, st, healthChecker, adminIDs, cfg.ChannelID)

	// Weekly scheduler
	weeklyScheduler := scheduler.NewWeeklyScheduler(weeklyReporter)
	weeklyScheduler.Start()
	defer func() {
		log.Println("Stopping weekly scheduler...")
		weeklyScheduler.Stop()
	}()

	// Планировщик
	sched := scheduler.NewScheduler(r)
	sched.Start()
	defer func() {
		log.Println("Stopping main scheduler...")
		sched.Stop()
	}()

	// Graceful shutdown при получении сигнала
	go func() {
		<-ctx.Done()
		log.Println("Received shutdown signal, stopping gracefully...")

		// Останавливаем планировщики
		weeklyScheduler.Stop()
		sched.Stop()

		// Закрываем БД
		if err := st.DB.Close(); err != nil {
			log.Printf("Error closing database: %v", err)
		}

		log.Println("Graceful shutdown completed")
	}()

	if cfg.UseWebhook {
		wh, err := tgbotapi.NewWebhook(cfg.WebhookURL)
		if err != nil {
			log.Fatalf("new webhook error: %v", err)
		}

		// В v5.5.1 SecretToken не поддерживается, поэтому пропускаем её установку
		if _, err = bot.Request(wh); err != nil {
			log.Fatalf("set webhook error: %v", err)
		}

		info, err := bot.GetWebhookInfo()
		if err != nil {
			log.Printf("GetWebhookInfo error: %v", err)
		} else {
			log.Printf("Webhook set: %s (last_error_date=%d, last_error_message=%q)",
				info.URL, info.LastErrorDate, info.LastErrorMessage)
		}

		// Подписываемся на входящие апдейты по пути
		updates := bot.ListenForWebhook(cfg.WebhookPath)
		srv := &http.Server{Addr: cfg.WebhookAddr, Handler: nil}
		go func() {
			log.Printf("HTTP listen on %s (path %s)", cfg.WebhookAddr, cfg.WebhookPath)
			if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				log.Fatalf("http server error: %v", err)
			}
		}()

		for {
			select {
			case <-ctx.Done():
				bot.StopReceivingUpdates()
				_ = srv.Shutdown(ctx)
				log.Println("shutdown: webhook server stopped")
				return
			case upd, ok := <-updates:
				if !ok {
					log.Println("updates channel closed")
					return
				}
				r.Handle(upd)
			}
		}
	} else {
		// getUpdates режим
		u := tgbotapi.NewUpdate(0)
		u.AllowedUpdates = []string{"message", "channel_post", "callback_query", "inline_query"}
		u.Timeout = 30
		updates := bot.GetUpdatesChan(u)
		for {
			select {
			case <-ctx.Done():
				bot.StopReceivingUpdates()
				log.Println("shutdown: stopped receiving updates")
				return
			case update, ok := <-updates:
				if !ok {
					log.Println("updates channel closed")
					return
				}
				r.Handle(update)
			}
		}
	}
}
