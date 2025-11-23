package router

import (
	"tgbot/internal/logger"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// handleClearDatabaseCommand обрабатывает команду /clear_db (только для тестирования!)
func (r *Router) handleClearDatabaseCommand(msg *tgbotapi.Message) {
	// Проверяем права доступа
	if !r.isAdmin(msg.From.ID) {
		return
	}

	// Очищаем базу данных
	if err := r.store.ClearAllOrders(); err != nil {
		logger.LogOrderError(err, 0, "clear database")
		reply := tgbotapi.NewMessage(msg.Chat.ID, "❌ Ошибка при очистке базы данных")
		if _, err := r.bot.Send(reply); err != nil {
			logger.LogSendError(err, msg.Chat.ID, "clear database error")
		}
		return
	}

	// Отправляем подтверждение
	reply := tgbotapi.NewMessage(msg.Chat.ID, "🗑️ База данных очищена! Все заявки удалены.\n\n⚠️ ВНИМАНИЕ: Эта команда только для тестирования!")
	if _, err := r.bot.Send(reply); err != nil {
		logger.LogSendError(err, msg.Chat.ID, "clear database confirmation")
	}
}
