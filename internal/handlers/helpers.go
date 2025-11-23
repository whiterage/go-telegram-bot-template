package router

import (
	"fmt"
	"log"
	"strings"

	"tgbot/internal/logger"
	"tgbot/internal/state"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

/* ===================== Подписка и утилиты ===================== */

func (r *Router) isSubscribed(userID int64) bool {
	cfg := tgbotapi.GetChatMemberConfig{
		ChatConfigWithUser: tgbotapi.ChatConfigWithUser{
			ChatID: r.channelID,
			UserID: userID,
		},
	}
	member, err := r.bot.GetChatMember(cfg)
	if err != nil {
		logger.LogRequestError(err, "getChatMember", fmt.Sprintf("user_id=%d", userID))
		return false
	}
	log.Printf("subscription check: user=%d status=%s is_member=%v", userID, member.Status, member.IsMember)

	switch member.Status {
	case "member":
		return true
	case "restricted":
		return member.IsMember // bool
	case "administrator", "creator":
		return r.allowAdminsBypass
	default:
		return false
	}
}

func (r *Router) sendGate(chatID int64) {
	btn := tgbotapi.NewInlineKeyboardButtonURL("🔔 Подписаться на канал", r.channelURL)
	inline := tgbotapi.NewInlineKeyboardMarkup(tgbotapi.NewInlineKeyboardRow(btn))
	m1 := tgbotapi.NewMessage(chatID, "Чтобы пользоваться ботом, подпишитесь на наш канал.")
	m1.ReplyMarkup = inline
	if _, err := r.bot.Send(m1); err != nil {
		logger.LogSendError(err, chatID, "subscription_gate")
	}

	kb := tgbotapi.NewReplyKeyboard(tgbotapi.NewKeyboardButtonRow(tgbotapi.NewKeyboardButton("Проверить подписку ✅")))
	kb.ResizeKeyboard = true
	m2 := tgbotapi.NewMessage(chatID, "После подписки нажмите «Проверить подписку ✅».")
	m2.ReplyMarkup = kb
	if _, err := r.bot.Send(m2); err != nil {
		logger.LogSendError(err, chatID, "subscription_check")
	}
}

func (r *Router) startQuestionnaire(chatID int64, sess *state.Session) {
	text := "Отлично! Начнём оформление.\nВыберите тип учреждения:"
	reply := tgbotapi.NewMessage(chatID, text)
	reply.ReplyMarkup = r.createInstitutionKeyboard()
	if _, err := r.bot.Send(reply); err != nil {
		logger.LogSendError(err, chatID, "questionnaire_start")
	}
	sess.Stage = state.StageChooseInstitution
}

func (r *Router) replyText(chatID int64, text string) {
	m := tgbotapi.NewMessage(chatID, text)
	if _, err := r.bot.Send(m); err != nil {
		logger.LogSendError(err, chatID, "reply_text")
	}
}

func (r *Router) buildSummary(sess *state.Session) string {
	attach := "нет"
	if len(sess.AttachIDs) > 0 {
		attach = fmt.Sprintf("%d файл(ов)", len(sess.AttachIDs))
	}

	source := strings.TrimSpace(sess.ClientSource)
	if source == "" {
		source = "Не указано"
	}

	return "📋 Сводка заявки\n\n" +
		"🏫 Тип учреждения: " + sess.InstitutionType + "\n" +
		"📅 Срок выполнения: " + sess.Deadline + "\n" +
		"🧩 Вид услуги: " + sess.WorkType + "\n" +
		"📄 Объём: " + sess.Pages + "\n" +
		"📎 Прикреплённые файлы: " + attach + "\n\n" +
		"📝 Описание:\n" + sess.Topic + "\n\n" +
		"📋 Дополнительная информация:\n" + sess.Requirements + "\n\n" +
		"👥 Источник:\n" + source + "\n\n" +
		"Всё верно?"
}

/* ===== ВСПОМОГАТЕЛЬНОЕ ===== */

func normalizePages(s string) (string, bool) {
	x := strings.TrimSpace(s)
	l := strings.ToLower(x)
	if l == "не знаю" || l == "не знаю." || l == "не знаю!" {
		return "не знаю", true
	}
	x = strings.ReplaceAll(x, "---", "–")
	x = strings.ReplaceAll(x, "--", "–")
	x = strings.ReplaceAll(x, "—", "–")
	x = strings.ReplaceAll(x, "-", "–")
	x = strings.ReplaceAll(x, " – ", "–")
	x = strings.ReplaceAll(x, " –", "–")
	x = strings.ReplaceAll(x, "– ", "–")
	if strings.Contains(x, "–") {
		parts := strings.Split(x, "–")
		if len(parts) != 2 {
			return "", false
		}
		left := strings.TrimSpace(parts[0])
		right := strings.TrimSpace(parts[1])
		if isUint(left) && isUint(right) {
			return left + "–" + right, true
		}
		return "", false
	}
	if isUint(x) {
		return x, true
	}
	return "", false
}

func isUint(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}
