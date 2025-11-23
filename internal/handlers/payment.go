package router

import (
	"fmt"
	"strconv"
	"strings"

	"tgbot/internal/constants"
	"tgbot/internal/logger"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// handlePaymentAmountInput обрабатывает ввод суммы платежа
func (r *Router) handlePaymentAmountInput(msg *tgbotapi.Message) {
	if !r.isAdmin(msg.From.ID) {
		return
	}

	p, ok := r.peekPendingAmount(msg.From.ID)
	if !ok {
		reply := tgbotapi.NewMessage(msg.Chat.ID, "Сначала нажмите «✅ Принять №…», потом введите сумму.")
		reply.ReplyToMessageID = msg.MessageID
		_, _ = r.bot.Send(reply)
		return
	}

	amountStr := strings.ReplaceAll(strings.TrimSpace(msg.Text), ",", ".")
	amount, err := strconv.ParseFloat(amountStr, 64)
	if err != nil || amount <= 0 {
		reply := tgbotapi.NewMessage(msg.Chat.ID, "❌ Неверный формат. Пример: 1500 или 1500.50")
		reply.ReplyToMessageID = msg.MessageID
		_, _ = r.bot.Send(reply)
		return
	}

	// 1) пишем сумму + дату
	if err := r.store.SetPaymentAmount(p.OrderID, amount); err != nil {
		logger.LogOrderError(err, p.OrderID, "set payment amount")
		_, _ = r.bot.Send(tgbotapi.NewMessage(msg.Chat.ID, "❌ Ошибка при сохранении суммы"))
		return
	}
	// 2) статус paid
	if err := r.store.SetStatus(p.OrderID, constants.StatusPaid); err != nil {
		logger.LogOrderError(err, p.OrderID, "set_status_paid")
		_, _ = r.bot.Send(tgbotapi.NewMessage(msg.Chat.ID, "❌ Ошибка при обновлении статуса"))
		return
	}

	// 3) двигаем карточку + сообщаем пользователю
	if o, err := r.store.LoadOrder(p.OrderID); err == nil {
		r.moveCard(o, r.topicPaidID)
		_, _ = r.bot.Send(tgbotapi.NewMessage(o.ChatID,
			fmt.Sprintf("Оплата по заявке №%d подтверждена. Статус: оплачен ✅", p.OrderID)))
	}

	// 4) гасим клавиатуру у исходного модерационного сообщения (если ещё есть)
	gray := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("✔ Обработано", "noop"),
		),
	)
	if _, err := r.bot.Send(tgbotapi.NewEditMessageReplyMarkup(p.ChatID, p.MsgID, gray)); err != nil {
		logger.LogSendError(err, p.ChatID, "edit_reply_markup_after_amount")
	}

	// 5) подтверждение админу
	r.takePendingAmount(msg.From.ID)
	conf := tgbotapi.NewMessage(msg.Chat.ID,
		fmt.Sprintf("✅ Оплата для №%d: %s ₽. Перенесено в «Оплаченные».", p.OrderID, formatAmount(amount)))
	conf.ReplyToMessageID = msg.MessageID
	_, _ = r.bot.Send(conf)
}

// formatAmount форматирует сумму для отображения
func formatAmount(amount float64) string {
	if amount == float64(int64(amount)) {
		return fmt.Sprintf("%.0f", amount)
	}
	return fmt.Sprintf("%.2f", amount)
}

// isAdmin проверяет, является ли пользователь админом
func (r *Router) isAdmin(userID int64) bool {
	_, isAdmin := r.adminIDs[userID]
	return isAdmin
}

func (r *Router) setPendingAmount(adminID, orderID int64, chatID int64, msgID int) {
	r.payLock.Lock()
	defer r.payLock.Unlock()
	r.pendingAmount[adminID] = pendingPay{OrderID: orderID, ChatID: chatID, MsgID: msgID}
}

func (r *Router) takePendingAmount(adminID int64) (pendingPay, bool) {
	r.payLock.Lock()
	defer r.payLock.Unlock()
	p, ok := r.pendingAmount[adminID]
	if ok {
		delete(r.pendingAmount, adminID)
	}
	return p, ok
}

func (r *Router) peekPendingAmount(adminID int64) (pendingPay, bool) {
	r.payLock.Lock()
	defer r.payLock.Unlock()
	p, ok := r.pendingAmount[adminID]
	return p, ok
}
