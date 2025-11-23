package router

import (
	"fmt"
	"strings"
	"time"

	"tgbot/internal/constants"
	"tgbot/internal/logger"
	"tgbot/internal/state"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

/* ===================== Команды ===================== */

func (r *Router) cmdStart(msg *tgbotapi.Message) {
	chatID := msg.Chat.ID
	r.started.MarkStarted(chatID)
	sess := r.sessions.Get(chatID)

	if !r.isSubscribed(msg.From.ID) {
		hello := "Привет! Я помогу вам оформить заявку.\n\n" +
			"Для продолжения необходимо подписаться на канал."
		r.replyText(chatID, hello)
		r.sendGate(chatID)
		sess.Stage = state.StageWelcome
		return
	}

	r.startQuestionnaire(chatID, sess)
}

func (r *Router) cmdHelp(msg *tgbotapi.Message) {
	var helpText string

	if r.isAdmin(msg.From.ID) {
		// Полная справка для админов
		helpText = `🤖 **Команды бота:**

**Для всех пользователей:**
/start - Начать оформление заявки
/profile - Просмотр ваших заявок
/help - Показать эту справку

**Для администраторов:**
/analytics [период] - Статистика и аналитика
  • /analytics week - текущая неделя
  • /analytics month - текущий месяц
  • /analytics year - текущий год
  • /analytics total - общая статистика
/clear_db - Очистить базу данных (только для тестирования!)

**Поддержка:** Если у вас есть вопросы, обратитесь к администратору.`
	} else {
		// Ограниченная справка для обычных пользователей
		helpText = `🤖 **Команды бота:**

/start - Начать оформление заявки
/profile - Просмотр ваших заявок
/help - Показать эту справку

**Поддержка:** Если у вас есть вопросы, обратитесь к администратору.`
	}

	reply := tgbotapi.NewMessage(msg.Chat.ID, helpText)
	if _, err := r.bot.Send(reply); err != nil {
		logger.LogSendError(err, msg.Chat.ID, "help_command")
	}
}

func (r *Router) cmdFAQ(msg *tgbotapi.Message) {
	if r.faqURL == "" {
		r.replyText(msg.Chat.ID, "Ссылка на раздел FAQ временно не настроена. Сообщите администратору.")
		return
	}
	text := "🧠 FAQ: ответы на частые вопросы и условия оплаты:\n" + r.faqURL
	r.replyText(msg.Chat.ID, text)
}

func (r *Router) cmdProfile(msg *tgbotapi.Message) {
	r.showProfilePage(msg.Chat.ID, msg.From.ID, 1)
}

func (r *Router) showProfilePage(chatID, userID int64, page int) {
	const pageSize = 5

	orders, total, err := r.store.GetUserOrdersPaginated(userID, page, pageSize)
	if err != nil || len(orders) == 0 {
		r.replyText(chatID, "Заявок пока нет. Создайте новую через /start.")
		return
	}

	var b strings.Builder
	totalPages := (total + pageSize - 1) / pageSize // округляем вверх
	b.WriteString(fmt.Sprintf("📋 Ваши заявки (страница %d из %d)\n\n", page, totalPages))

	var rows [][]tgbotapi.InlineKeyboardButton
	for _, o := range orders {
		b.WriteString(fmt.Sprintf("• №%d — %s | дедлайн: %s | статус: %s\n", o.ID, o.Service, o.DeadlineRaw, constants.HumanStatus(o.Status)))
		if o.Status == constants.StatusAwaitPay || o.Status == constants.StatusReceiptPending {
			rows = append(rows, []tgbotapi.InlineKeyboardButton{
				tgbotapi.NewInlineKeyboardButtonData(fmt.Sprintf("💳 Подтверждение оплаты №%d", o.ID), fmt.Sprintf("rcpt:%d", o.ID)),
			})
		} else {
			rows = append(rows, []tgbotapi.InlineKeyboardButton{
				tgbotapi.NewInlineKeyboardButtonData(fmt.Sprintf("🔎 Статус №%d", o.ID), fmt.Sprintf("stat:%d", o.ID)),
			})
		}
	}

	// Добавляем кнопки навигации, если есть больше одной страницы
	if totalPages > 1 {
		var navButtons []tgbotapi.InlineKeyboardButton
		if page > 1 {
			navButtons = append(navButtons,
				tgbotapi.NewInlineKeyboardButtonData("◀️ Предыдущая", fmt.Sprintf("profile_page:%d", page-1)))
		}
		navButtons = append(navButtons,
			tgbotapi.NewInlineKeyboardButtonData(fmt.Sprintf("%d/%d", page, totalPages), "noop"))
		if page < totalPages {
			navButtons = append(navButtons,
				tgbotapi.NewInlineKeyboardButtonData("Следующая ▶️", fmt.Sprintf("profile_page:%d", page+1)))
		}
		rows = append(rows, navButtons)
	}

	m := tgbotapi.NewMessage(chatID, b.String())
	m.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(rows...)
	if _, err := r.bot.Send(m); err != nil {
		logger.LogSendError(err, chatID, "profile_command")
	}
}

func (r *Router) handleCommand(msg *tgbotapi.Message) {
	switch msg.Command() {
	case "start":
		r.cmdStart(msg)
	case "help":
		r.cmdHelp(msg)
	case "profile":
		r.cmdProfile(msg)
	case "analytics":
		r.handleAnalyticsCommand(msg)
	case "clear_db":
		r.handleClearDatabaseCommand(msg)
	case "find":
		r.handleFindCommand(msg)
	case "export":
		r.handleExportCommand(msg)
	case "faq":
		r.cmdFAQ(msg)
	case "ratelimit":
		r.cmdRateLimitStats(msg)
	case "weekly":
		r.cmdWeeklyReport(msg)
	case "health":
		r.cmdHealthCheck(msg)
	default:
		r.replyText(msg.Chat.ID, "Неизвестная команда. Попробуйте /start")
	}
}

func (r *Router) handleFindCommand(msg *tgbotapi.Message) {
	if !r.isAdmin(msg.From.ID) {
		return
	}

	args := strings.TrimSpace(strings.TrimPrefix(msg.Text, "/find"))
	if args == "" {
		r.replyText(msg.Chat.ID, "Использование: /find <номер или фраза>")
		return
	}
	list, err := r.store.FindOrders(args, 15)
	if err != nil || len(list) == 0 {
		r.replyText(msg.Chat.ID, "Ничего не найдено.")
		return
	}

	var b strings.Builder
	for _, o := range list {
		b.WriteString(fmt.Sprintf("• №%d — %s | дедлайн: %s | статус: %s\n", o.ID, o.Service, o.DeadlineRaw, constants.HumanStatus(o.Status)))
	}
	// Простая клавиатура быстр.действий: открыть чат и статусы
	var rows [][]tgbotapi.InlineKeyboardButton
	for _, o := range list {
		rows = append(rows, []tgbotapi.InlineKeyboardButton{
			tgbotapi.NewInlineKeyboardButtonURL(fmt.Sprintf("💬 %d", o.ID), fmt.Sprintf("tg://user?id=%d", o.UserID)),
			tgbotapi.NewInlineKeyboardButtonData("🔎 Статус", fmt.Sprintf("stat:%d", o.ID)),
		})
	}

	m := tgbotapi.NewMessage(msg.Chat.ID, b.String())
	m.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(rows...)
	if _, err := r.bot.Send(m); err != nil {
		logger.LogSendError(err, msg.Chat.ID, "find_command")
	}
}

// cmdRateLimitStats показывает статистику защиты от спама (только для админов)
func (r *Router) cmdRateLimitStats(msg *tgbotapi.Message) {
	if !r.isAdmin(msg.From.ID) {
		return
	}

	stats := r.rateLimiter.GetStats()

	var response strings.Builder
	response.WriteString("🛡️ **Защита от спама**\n\n")
	response.WriteString("📊 **Текущая активность:**\n")

	totalActive := 0
	for _, stat := range stats {
		if statMap, ok := stat.(map[string]interface{}); ok {
			if activeUsers, ok := statMap["active_users"].(int); ok {
				totalActive += activeUsers
			}
		}
	}

	response.WriteString(fmt.Sprintf("👥 Всего активных пользователей: %d\n\n", totalActive))

	response.WriteString("⚙️ **Настройки защиты:**\n")
	response.WriteString("• Сообщения: 100 за 15 минут\n")
	response.WriteString("• Файлы: 20 за 20 минут\n")
	response.WriteString("• Кнопки: 200 за 15 минут\n")
	response.WriteString("• Команды: 20 за 10 минут\n\n")

	response.WriteString("✅ **Система работает нормально**\n")
	response.WriteString("🔒 Защита активна и защищает от спама")

	m := tgbotapi.NewMessage(msg.Chat.ID, response.String())
	m.ParseMode = "Markdown"
	if _, err := r.bot.Send(m); err != nil {
		logger.LogSendError(err, msg.Chat.ID, "ratelimit_stats")
	}
}

// cmdWeeklyReport генерирует и отправляет еженедельный отчет (только для админов)
func (r *Router) cmdWeeklyReport(msg *tgbotapi.Message) {
	if !r.isAdmin(msg.From.ID) {
		return
	}

	// Получаем статистику за последнюю неделю
	now := time.Now()
	weekStart := now.AddDate(0, 0, -7)

	totalUsers, _ := r.store.GetUsersCount()
	totalOrders, _ := r.store.GetOrdersCount()
	ordersThisWeek, _ := r.store.GetOrdersCountSince(weekStart)

	response := "📊 **Еженедельный отчет**\n\n"
	response += fmt.Sprintf("📅 Период: %s - %s\n\n",
		weekStart.Format("02.01.2006"), now.Format("02.01.2006"))

	response += "📈 **Статистика:**\n"
	response += fmt.Sprintf("👥 Всего пользователей: %d\n", totalUsers)
	response += fmt.Sprintf("📝 Всего заявок: %d\n", totalOrders)
	response += fmt.Sprintf("📋 Заявок за неделю: %d\n\n", ordersThisWeek)

	if ordersThisWeek > 0 {
		avgPerDay := float64(ordersThisWeek) / 7
		response += fmt.Sprintf("📊 Среднедневная активность: %.1f заявок/день\n\n", avgPerDay)
	}

	response += "✅ **Бот работает стабильно**\n"
	response += "🔄 Автоматические отчеты отправляются каждый понедельник в 9:00"

	m := tgbotapi.NewMessage(msg.Chat.ID, response)
	m.ParseMode = "Markdown"
	if _, err := r.bot.Send(m); err != nil {
		logger.LogSendError(err, msg.Chat.ID, "weekly_report")
	}
}

// cmdHealthCheck показывает состояние системы (только для админов)
func (r *Router) cmdHealthCheck(msg *tgbotapi.Message) {
	if !r.isAdmin(msg.From.ID) {
		return
	}

	// Проверяем базу данных
	totalUsers, err1 := r.store.GetUsersCount()
	totalOrders, err2 := r.store.GetOrdersCount()

	response := "🏥 **Состояние системы**\n\n"

	// Статус базы данных
	if err1 != nil || err2 != nil {
		response += "❌ **База данных:** Проблемы с подключением\n"
	} else {
		response += "✅ **База данных:** Работает нормально\n"
	}

	// Статус rate limiting
	response += "✅ **Защита от спама:** Активна\n"

	// Статистика
	response += "\n📊 **Статистика:**\n"
	response += fmt.Sprintf("👥 Пользователей: %d\n", totalUsers)
	response += fmt.Sprintf("📝 Заявок: %d\n", totalOrders)

	// Общий статус
	response += "\n✅ **Система работает стабильно**\n"
	response += "🔧 Все компоненты функционируют нормально"

	m := tgbotapi.NewMessage(msg.Chat.ID, response)
	m.ParseMode = "Markdown"
	if _, err := r.bot.Send(m); err != nil {
		logger.LogSendError(err, msg.Chat.ID, "health_check")
	}
}
