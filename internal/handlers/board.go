package router

import (
	"encoding/json"
	"fmt"
	"html"
	"strconv"

	"tgbot/internal/constants"
	"tgbot/internal/logger"
	"tgbot/internal/storage"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

/* ===================== Board Management ===================== */

func (r *Router) orderCardHTML(o *storage.Order) string {
	name := fmt.Sprintf("tg://user?id=%d", o.UserID)
	return fmt.Sprintf(
		"📥 <b>Новая заявка №%d</b>\n\n"+
			"👤 <b>Пользователь:</b> <a href=\"%s\">открыть чат</a>\n"+
			"🧩 <b>Вид услуги:</b> %s\n"+
			"📅 <b>Срок выполнения:</b> %s\n"+
			"📄 <b>Объём:</b> %s\n\n"+
			"📝 <b>Описание:</b>\n<pre>%s</pre>\n\n"+
			"📋 <b>Дополнительная информация:</b>\n<pre>%s</pre>\n"+
			"👥 <b>Источник:</b> %s\n"+
			"🏷 <b>Статус:</b> %s",
		o.ID, name, html.EscapeString(o.Service),
		html.EscapeString(o.DeadlineRaw), html.EscapeString(o.Pages),
		html.EscapeString(o.Topic), html.EscapeString(o.Requirements), html.EscapeString(o.ClientSource), constants.HumanStatus(o.Status),
	)
}

func (r *Router) boardButtonsFor(o *storage.Order, threadID int) tgbotapi.InlineKeyboardMarkup {
	open := tgbotapi.NewInlineKeyboardButtonURL("💬 Открыть чат", fmt.Sprintf("tg://user?id=%d", o.UserID))
	idbtn := tgbotapi.NewInlineKeyboardButtonData(fmt.Sprintf("№%d", o.ID), "noop")
	if threadID == r.topicPaidID || o.Status == constants.StatusPaid {
		done := tgbotapi.NewInlineKeyboardButtonData(fmt.Sprintf("✅ Завершить №%d", o.ID), fmt.Sprintf("doneok:%d", o.ID))
		undo := tgbotapi.NewInlineKeyboardButtonData(fmt.Sprintf("❌ Отклонить №%d", o.ID), fmt.Sprintf("doneno:%d", o.ID))
		return tgbotapi.NewInlineKeyboardMarkup(
			tgbotapi.NewInlineKeyboardRow(open, idbtn),
			tgbotapi.NewInlineKeyboardRow(done, undo),
		)
	}
	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(open, idbtn),
	)
}

func (r *Router) sendMessageToThread(chatID int64, threadID int, text string, kb *tgbotapi.InlineKeyboardMarkup) (int, error) {
	params := tgbotapi.Params{}
	params["chat_id"] = strconv.FormatInt(chatID, 10)
	params["text"] = text
	params["message_thread_id"] = strconv.Itoa(threadID)
	params["parse_mode"] = "HTML" // ← вот это
	if kb != nil {
		b, _ := json.Marshal(kb)
		params["reply_markup"] = string(b)
	}

	resp, err := r.bot.MakeRequest("sendMessage", params)
	if err != nil {
		return 0, err
	}
	var m tgbotapi.Message
	if err := json.Unmarshal(resp.Result, &m); err != nil {
		return 0, err
	}
	return m.MessageID, nil
}

// отправка ФОТО в конкретную тему
func (r *Router) sendPhotoToThread(chatID int64, threadID int, fileID string, caption string, kb *tgbotapi.InlineKeyboardMarkup) (int, error) {
	params := tgbotapi.Params{
		"chat_id":           strconv.FormatInt(chatID, 10),
		"message_thread_id": strconv.Itoa(threadID),
		"photo":             fileID, // ВАЖНО: передаём file_id, НЕ файлы
		"caption":           caption,
	}
	if kb != nil {
		b, _ := json.Marshal(kb)
		params["reply_markup"] = string(b)
	}
	resp, err := r.bot.MakeRequest("sendPhoto", params)
	if err != nil {
		return 0, err
	}
	var m tgbotapi.Message
	if err := json.Unmarshal(resp.Result, &m); err != nil {
		return 0, err
	}
	return m.MessageID, nil
}

// Документ в тему forum (через file_id)
func (r *Router) sendDocumentToThread(chatID int64, threadID int, fileID string, caption string, kb *tgbotapi.InlineKeyboardMarkup) (int, error) {
	params := tgbotapi.Params{
		"chat_id":           strconv.FormatInt(chatID, 10),
		"message_thread_id": strconv.Itoa(threadID),
		"document":          fileID, // ВАЖНО: file_id
		"caption":           caption,
	}
	if kb != nil {
		b, _ := json.Marshal(kb)
		params["reply_markup"] = string(b)
	}
	resp, err := r.bot.MakeRequest("sendDocument", params)
	if err != nil {
		return 0, err
	}
	var m tgbotapi.Message
	if err := json.Unmarshal(resp.Result, &m); err != nil {
		return 0, err
	}
	return m.MessageID, nil
}

func (r *Router) postToTopic(o *storage.Order, threadID int) (int, error) {
	kb := r.boardButtonsFor(o, threadID)
	msgID, err := r.sendMessageToThread(r.boardChatID, threadID, r.orderCardHTML(o), &kb)
	if err != nil {
		return 0, err
	}
	_ = r.store.UpdateBoardPost(o.ID, int64(msgID), threadID)
	return msgID, nil
}

func (r *Router) moveCard(o *storage.Order, newThread int) {
	oldMsgID, oldThread, _ := r.store.GetBoardPost(o.ID)
	if oldMsgID != 0 && oldThread != 0 {
		del := tgbotapi.NewDeleteMessage(r.boardChatID, int(oldMsgID))
		if _, err := r.bot.Request(del); err != nil {
			// теперь просто лог и идём дальше
			logger.LogBoardError(err, o.ID, "delete_old_card", oldThread)
		}
	}
	if _, err := r.postToTopic(o, newThread); err != nil {
		logger.LogBoardError(err, o.ID, "post_new_card", newThread)
	}
}

func (r *Router) updateCardStatus(o *storage.Order) {
	msgID, threadID, err := r.store.GetBoardPost(o.ID)
	if err != nil || msgID == 0 {
		return
	}
	newText := r.orderCardHTML(o)
	edit := tgbotapi.NewEditMessageText(r.boardChatID, int(msgID), newText)
	edit.ParseMode = "HTML"
	kb := r.boardButtonsFor(o, threadID)
	edit.ReplyMarkup = &kb

	if _, err := r.bot.Send(edit); err != nil {
		logger.LogBoardError(err, o.ID, "update_card_text", threadID)
	}
}
