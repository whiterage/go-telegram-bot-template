package router

import (
	"fmt"

	"tgbot/internal/logger"
	"tgbot/internal/state"
	"tgbot/internal/validation"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

/* ===================== Загрузка чека ===================== */

func (r *Router) handleReceiptUpload(msg *tgbotapi.Message, sess *state.Session) {
	chatID := msg.Chat.ID
	oid := sess.AwaitReceiptFor
	if msg.Document == nil && len(msg.Photo) == 0 {
		r.replyText(chatID, "Пришлите фото или PDF-файл подтверждения оплаты.")
		return
	}

	var fileID, fileName string
	var fileSize int64
	var mimeType string

	if msg.Document != nil {
		fileID = msg.Document.FileID
		fileName = msg.Document.FileName
		fileSize = int64(msg.Document.FileSize)
		mimeType = msg.Document.MimeType
	} else {
		p := msg.Photo[len(msg.Photo)-1]
		fileID = p.FileID
		fileName = "photo.jpg" // Фото не имеют имени файла
		fileSize = int64(p.FileSize)
		mimeType = "image/jpeg"
	}

	// Валидация файла
	if err := validation.ValidateReceiptFile(fileName, fileSize, mimeType); err != nil {
		r.replyText(chatID, fmt.Sprintf("❌ %s", err.Error()))
		return
	}

	typ := validation.GetFileTypeFromName(fileName)

	if err := r.store.SaveReceipt(oid, fileID, typ); err != nil {
		r.replyText(chatID, "Не удалось сохранить файл. Попробуйте ещё раз.")
		return
	}

	// В ревью-чат: файл + кнопки модерации
	threadID := r.topicInProgressID
	if msgID, tID, err := r.store.GetBoardPost(oid); err == nil && msgID != 0 && tID != 0 {
		threadID = tID
	}

	caption := fmt.Sprintf("🧾 Подтверждение оплаты по заявке №%d\nОт: @%s (%d)\nСтатус: на модерации",
		oid, msg.From.UserName, msg.From.ID)

	ik := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(fmt.Sprintf("✅ Принять №%d", oid), fmt.Sprintf("payok:%d", oid)),
			tgbotapi.NewInlineKeyboardButtonData(fmt.Sprintf("❌ Отклонить №%d", oid), fmt.Sprintf("payno:%d", oid)),
		),
	)

	var err error
	if typ == "photo" {
		_, err = r.sendPhotoToThread(r.boardChatID, threadID, fileID, caption, &ik)
	} else {
		_, err = r.sendDocumentToThread(r.boardChatID, threadID, fileID, caption, &ik)
	}
	if err != nil {
		logger.LogBoardError(err, oid, "send_receipt", threadID)
	}

	r.replyText(chatID, fmt.Sprintf("Файл принят и отправлен на модерацию для заявки №%d. Обычно обработка занимает немного времени 🙌", oid))
	sess.Stage = state.StageIdle
	sess.AwaitReceiptFor = 0
}
