package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

type AppConfig struct {
	BotToken string
	Env      string

	ChannelID         int64
	ChannelURL        string
	AllowAdminsBypass bool

	AdminIDsCSV       string
	BoardChatID       int64
	TopicInProgressID int
	TopicPaidID       int
	TopicDoneID       int
	DeadlineTopicID   int
	DBPath            string
	UseWebhook        bool
	WebhookURL        string
	WebhookAddr       string
	WebhookPath       string
	WebhookSecret     string
	FAQURL            string
}

func Load() (AppConfig, error) {
	// Загружаем .env файл если он существует
	if err := godotenv.Load(); err != nil {
		// Игнорируем ошибку если файл не найден
		fmt.Printf("Warning: .env file not found, using system environment variables\n")
	}

	cfg := AppConfig{}

	cfg.FAQURL = strings.TrimSpace(os.Getenv("FAQ_URL"))

	allow := strings.TrimSpace(os.Getenv("ALLOW_ADMINS_BYPASS"))
	// по умолчанию TRUE (пропускать админов/создателя)
	cfg.AllowAdminsBypass = !(allow == "0" || strings.EqualFold(allow, "false") || strings.EqualFold(allow, "no"))

	cfg.BotToken = strings.TrimSpace(os.Getenv("BOT_TOKEN"))
	cfg.Env = strings.TrimSpace(os.Getenv("APP_ENV"))

	if cfg.Env == "" {
		cfg.Env = "dev"
	}

	if cfg.BotToken == "" {
		return AppConfig{}, errors.New("BOT_TOKEN is empty: set env var BOT_TOKEN")
	}

	chRaw := strings.TrimSpace(os.Getenv("CHANNEL_ID"))
	if chRaw == "" {
		return AppConfig{}, errors.New("CHANNEL_ID is empty: set env var CHANNEL_ID (numeric channel id like -100...)")
	}
	chID, err := strconv.ParseInt(chRaw, 10, 64)
	if err != nil {
		return AppConfig{}, errors.New("CHANNEL_ID must be an integer (like -1001234567890)")
	}
	cfg.ChannelID = chID

	cfg.ChannelURL = strings.TrimSpace(os.Getenv("CHANNEL_URL"))
	if cfg.ChannelURL == "" {
		return AppConfig{}, errors.New("CHANNEL_URL is empty: set env var CHANNEL_URL (t.me/...)")
	}
	cfg.AdminIDsCSV = strings.TrimSpace(os.Getenv("ADMIN_IDS"))
	boardRaw := strings.TrimSpace(os.Getenv("BOARD_CHAT_ID"))
	if boardRaw == "" {
		return AppConfig{}, errors.New("BOARD_CHAT_ID is empty")
	}
	boardID, err := strconv.ParseInt(boardRaw, 10, 64)
	if err != nil {
		return AppConfig{}, errors.New("BOARD_CHAT_ID must be int64")
	}
	cfg.BoardChatID = boardID

	parseInt := func(env, name string) (int, error) {
		v := strings.TrimSpace(os.Getenv(env))
		if v == "" {
			return 0, fmt.Errorf("%s is empty", env)
		}
		n, err := strconv.Atoi(v)
		if err != nil {
			return 0, fmt.Errorf("%s must be int", env)
		}
		return n, nil
	}
	if cfg.TopicInProgressID, err = parseInt("INPROGRESS_TOPIC_ID", "INPROGRESS_TOPIC_ID"); err != nil {
		return AppConfig{}, err
	}
	if cfg.TopicPaidID, err = parseInt("PAID_TOPIC_ID", "PAID_TOPIC_ID"); err != nil {
		return AppConfig{}, err
	}
	if cfg.TopicDoneID, err = parseInt("DONE_TOPIC_ID", "DONE_TOPIC_ID"); err != nil {
		return AppConfig{}, err
	}
	if cfg.DeadlineTopicID, err = parseInt("DEADLINE_TOPIC_ID", "DEADLINE_TOPIC_ID"); err != nil {
		return AppConfig{}, err
	}
	cfg.DBPath = strings.TrimSpace(os.Getenv("DB_PATH"))
	if cfg.DBPath == "" {
		cfg.DBPath = "data.db"
	}
	uw := strings.TrimSpace(os.Getenv("USE_WEBHOOK"))
	cfg.UseWebhook = strings.EqualFold(uw, "1") || strings.EqualFold(uw, "true") || strings.EqualFold(uw, "yes")

	cfg.WebhookURL = strings.TrimSpace(os.Getenv("WEBHOOK_URL"))
	cfg.WebhookSecret = strings.TrimSpace(os.Getenv("WEBHOOK_SECRET"))
	cfg.WebhookAddr = strings.TrimSpace(os.Getenv("WEBHOOK_ADDR"))
	if cfg.WebhookAddr == "" {
		cfg.WebhookAddr = ":8080"
	}
	cfg.WebhookPath = strings.TrimSpace(os.Getenv("WEBHOOK_PATH"))
	if cfg.WebhookPath == "" {
		cfg.WebhookPath = "/telegram/webhook"
	}

	if cfg.UseWebhook {
		if cfg.WebhookURL == "" {
			return AppConfig{}, errors.New("USE_WEBHOOK=true, но WEBHOOK_URL пуст")
		}
	}

	return cfg, nil
}
