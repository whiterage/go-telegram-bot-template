package router

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"tgbot/internal/constants"
	"tgbot/internal/logger"
	"tgbot/internal/state"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

/* ===================== CallbackQuery ===================== */

func (r *Router) handleCallback(cq *tgbotapi.CallbackQuery) {
	data := cq.Data

	// Разрешаем callback-и:
	// - из лички (private),
	// - из ревью-чата (reviewChatID),
	// - из борды (boardChatID) — пригодится для done-кнопок.
	if cq.Message != nil && cq.Message.Chat != nil && cq.Message.Chat.Type != "private" {
		if cq.Message.Chat.ID != r.boardChatID {
			return
		}
	}

	if strings.HasPrefix(data, "form_") {
		r.handleFormCallback(cq)
		return
	}

	// ----- stat: показать статус -----
	if strings.HasPrefix(data, "stat:") {
		r.handleStatusCallback(cq)
		return
	}

	// ----- rcpt: запросить загрузку чека -----
	if strings.HasPrefix(data, "rcpt:") {
		r.handleReceiptCallback(cq)
		return
	}

	// ----- profile_page: пагинация профиля -----
	if strings.HasPrefix(data, "profile_page:") {
		r.handleProfilePageCallback(cq)
		return
	}

	// ----- payok / payno: модерация оплаты в ревью-чате -----
	if strings.HasPrefix(data, "payok:") || strings.HasPrefix(data, "payno:") {
		r.handlePaymentModerationCallback(cq)
		return
	}

	if strings.HasPrefix(data, "doneok:") || strings.HasPrefix(data, "doneno:") {
		r.handleCompletionCallback(cq)
		return
	}
}

func (r *Router) handleFormCallback(cq *tgbotapi.CallbackQuery) {
	if cq.Message == nil || cq.Message.Chat == nil {
		return
	}
	if cq.Message.Chat.Type != "private" {
		return
	}

	defer func() {
		if _, err := r.bot.Request(tgbotapi.NewCallback(cq.ID, "")); err != nil {
			logger.LogCallbackError(err, cq.ID, cq.From.ID)
		}
	}()

	chatID := cq.Message.Chat.ID
	r.clearInlineKeyboard(chatID, cq.Message.MessageID)
	data := cq.Data
	sess := r.sessions.Get(chatID)

	switch {
	case data == cbInstVuz:
		sess.InstitutionType = "Вуз"
		r.navigateToCategory(chatID)
	case data == cbInstCollege:
		sess.InstitutionType = "Колледж"
		r.navigateToCategory(chatID)
	case data == cbInstOther:
		sess.InstitutionType = "Другое"
		r.navigateToCategory(chatID)
	case data == cbBackInstitution:
		r.navigateToInstitution(chatID)
	case data == cbCatMain:
		r.navigateToWorkType(chatID, "Основные услуги")
	case data == cbCatExtra:
		r.navigateToWorkType(chatID, "Доп. Услуги")
	case data == cbBackCategories:
		r.navigateToCategory(chatID)
	case strings.HasPrefix(data, cbPrefixWorkMain):
		r.handleWorkTypeSelection(chatID, sess, data, cbPrefixWorkMain, mainWorkTypes, "Основные услуги")
	case strings.HasPrefix(data, cbPrefixWorkExtra):
		r.handleWorkTypeSelection(chatID, sess, data, cbPrefixWorkExtra, extraWorkTypes, "Доп. Услуги")
	case data == cbDeadlineUnknown:
		sess.Deadline = "не определён"
		r.navigateToPages(chatID)
	case data == cbBackWorkType:
		category := sess.WorkCategory
		if category == "" {
			category = "Основные услуги"
		}
		r.navigateToWorkType(chatID, category)
	case data == cbBackDeadline:
		r.navigateToDeadline(chatID)
	case data == cbPagesUnknown:
		sess.Pages = "пока не определился"
		r.navigateToTopic(chatID)
	case data == cbBackPages:
		r.navigateToPages(chatID)
	case data == cbBackTopic:
		r.navigateToTopic(chatID)
	case data == cbDocsReady:
		r.navigateToClientSource(chatID)
	case data == cbBackDocs:
		r.navigateToDocs(chatID)
	case data == cbConfirmSend:
		r.handleConfirmSendAction(chatID, cq.From.ID, sess)
	case data == cbConfirmEdit:
		r.handleConfirmEditAction(chatID, sess)
	case data == cbConfirmRestart:
		r.handleConfirmRestartAction(chatID)
	}
}

func (r *Router) handleWorkTypeSelection(chatID int64, sess *state.Session, data, prefix string, options []string, category string) {
	idxStr := strings.TrimPrefix(data, prefix)
	idx, err := strconv.Atoi(idxStr)
	if err != nil || idx < 0 || idx >= len(options) {
		return
	}

	sess.WorkCategory = category
	sess.WorkType = options[idx]
	r.navigateToDeadline(chatID)
}

func (r *Router) handleStatusCallback(cq *tgbotapi.CallbackQuery) {
	oid, _ := strconv.ParseInt(strings.TrimPrefix(cq.Data, "stat:"), 10, 64)

	// всегда убираем «часики»
	_, _ = r.bot.Request(tgbotapi.NewCallback(cq.ID, ""))

	o, err := r.store.LoadOrder(oid)
	if err != nil {
		// если нечего редактировать — просто пришлём новое сообщение
		if cq.Message != nil {
			r.replyText(cq.Message.Chat.ID, "Заявка не найдена.")
		} else {
			r.replyText(cq.From.ID, "Заявка не найдена.")
		}
		return
	}

	text := fmt.Sprintf(
		"№%d — %s\n📅 Дедлайн: %s\n🏷 Статус: %s",
		o.ID, o.Service, o.DeadlineRaw, constants.HumanStatus(o.Status),
	)
	if o.PaymentAmount > 0 {
		ts := time.Unix(o.PaymentDate, 0).Format("02.01.2006 15:04")
		text += fmt.Sprintf("\n💰 Оплата: %.2f ₽ (%s)", o.PaymentAmount, ts)
	}

	// если колбэк пришёл с кнопки под сообщением — редактируем это сообщение
	if cq.Message != nil {
		ed := tgbotapi.NewEditMessageText(cq.Message.Chat.ID, cq.Message.MessageID, text)
		if _, err := r.bot.Send(ed); err != nil {
			// если редактирование не удалось (например, чужое сообщение) — отправим новое
			r.replyText(cq.Message.Chat.ID, text)
		}
		return
	}

	// иначе отправим новое сообщение в личку
	r.replyText(cq.From.ID, text)
}

func (r *Router) handleReceiptCallback(cq *tgbotapi.CallbackQuery) {
	oid, _ := strconv.ParseInt(strings.TrimPrefix(cq.Data, "rcpt:"), 10, 64)
	if cq.Message == nil || cq.Message.Chat == nil {
		return
	}
	chatID := cq.Message.Chat.ID
	sess := r.sessions.Get(chatID)
	sess.Stage = state.StageAwaitReceiptUpload
	sess.AwaitReceiptFor = oid

	if _, err := r.bot.Request(tgbotapi.NewCallback(cq.ID, "Загрузите фото или PDF подтверждения оплаты")); err != nil {
		logger.LogCallbackError(err, cq.ID, cq.From.ID)
	}
	if _, err := r.bot.Send(tgbotapi.NewMessage(chatID,
		fmt.Sprintf("Пришлите фото или PDF подтверждения оплаты для заявки №%d.\nЧтобы отменить — /profile", oid))); err != nil {
		logger.LogSendError(err, chatID, "receipt_instructions")
	}
}

func (r *Router) handlePaymentModerationCallback(cq *tgbotapi.CallbackQuery) {
	if cq.Message == nil || cq.Message.Chat == nil {
		return
	}
	// Разрешаем только в ревью-чате или тем, кто в ADMIN_IDS
	allowed := (cq.Message.Chat.ID == r.boardChatID)
	if !allowed && len(r.adminIDs) > 0 {
		if _, ok := r.adminIDs[cq.From.ID]; ok {
			allowed = true
		}
	}
	if !allowed {
		if _, err := r.bot.Request(tgbotapi.NewCallback(cq.ID, "Недостаточно прав")); err != nil {
			logger.LogCallbackError(err, cq.ID, cq.From.ID)
		}
		return
	}

	accept := strings.HasPrefix(cq.Data, "payok:")
	idstr := strings.TrimPrefix(cq.Data, "payok:")
	if !accept {
		idstr = strings.TrimPrefix(cq.Data, "payno:")
	}
	oid, _ := strconv.ParseInt(idstr, 10, 64)

	// Проверяем текущий статус для идемпотентности
	order, err := r.store.LoadOrder(oid)
	if err != nil {
		logger.LogOrderError(err, oid, "load_order_for_payment")
		return
	}

	// Если уже обработано - отвечаем "уже обработано"
	if order.Status == constants.StatusPaid || order.Status == constants.StatusRejected {
		if _, err := r.bot.Request(tgbotapi.NewCallback(cq.ID, "Уже обработано")); err != nil {
			logger.LogCallbackError(err, cq.ID, cq.From.ID)
		}
		return
	}

	if accept {
		// ✅ добавь это:
		r.setPendingAmount(cq.From.ID, oid, cq.Message.Chat.ID, cq.Message.MessageID)

		// и попроси сумму с номером заявки:
		reply := tgbotapi.NewMessage(cq.Message.Chat.ID,
			fmt.Sprintf("💰 Введите сумму платежа для заявки №%d (например: 1500 или 1500.50):", oid))
		reply.ReplyToMessageID = cq.Message.MessageID
		if _, err := r.bot.Send(reply); err != nil {
			logger.LogSendError(err, cq.Message.Chat.ID, "payment amount request")
		}

		// ответ на callback
		if _, err := r.bot.Request(tgbotapi.NewCallback(cq.ID, "Введите сумму платежа")); err != nil {
			logger.LogCallbackError(err, cq.ID, cq.From.ID)
		}
		return
	} else {
		if err := r.store.SetStatus(oid, constants.StatusRejected); err != nil {
			logger.LogOrderError(err, oid, "set_status_rejected")
		}
		if o, err := r.store.LoadOrder(oid); err == nil {
			r.moveCard(o, r.topicInProgressID) // обратно "в обработке"
			r.updateCardStatus(o)              // обновляем статус в тексте
		}
	}

	// Серое "✔ Обработано" на исходном сообщении
	newKb := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("✔ Обработано", "noop"),
		),
	)
	edit := tgbotapi.NewEditMessageReplyMarkup(cq.Message.Chat.ID, cq.Message.MessageID, newKb)
	if _, err := r.bot.Send(edit); err != nil {
		logger.LogSendError(err, cq.Message.Chat.ID, "edit_reply_markup")
	}

	// Ответ на коллбек
	if _, err := r.bot.Request(tgbotapi.NewCallback(cq.ID, "Готово")); err != nil {
		logger.LogCallbackError(err, cq.ID, cq.From.ID)
	}

	// Уведомление пользователю
	if o, err := r.store.LoadOrder(oid); err == nil {
		var text string
		if accept {
			text = fmt.Sprintf("Оплата по заявке №%d подтверждена. Статус: оплачен ✅", oid)
		} else {
			text = fmt.Sprintf("Оплата по заявке №%d отклонена. Пожалуйста, загрузите корректное подтверждение оплаты через /profile.", oid)
		}
		if _, err := r.bot.Send(tgbotapi.NewMessage(o.ChatID, text)); err != nil {
			logger.LogSendError(err, o.ChatID, "notification")
		}
	}
}

func (r *Router) handleCompletionCallback(cq *tgbotapi.CallbackQuery) {
	// разрешаем только в борде или ADMIN_IDS (по аналогии с payok/payno)
	allowed := (cq.Message.Chat.ID == r.boardChatID)
	if !allowed && len(r.adminIDs) > 0 {
		if _, ok := r.adminIDs[cq.From.ID]; ok {
			allowed = true
		}
	}
	if !allowed {
		r.bot.Request(tgbotapi.NewCallback(cq.ID, "Недостаточно прав"))
		return
	}

	complete := strings.HasPrefix(cq.Data, "doneok:")
	idstr := strings.TrimPrefix(cq.Data, "doneok:")
	if !complete {
		idstr = strings.TrimPrefix(cq.Data, "doneno:")
	}
	oid, _ := strconv.ParseInt(idstr, 10, 64)

	// Проверяем текущий статус для идемпотентности
	order, err := r.store.LoadOrder(oid)
	if err != nil {
		logger.LogOrderError(err, oid, "load_order_for_completion")
		return
	}

	// Если уже завершено - отвечаем "уже обработано"
	if order.Status == constants.StatusDone {
		if _, err := r.bot.Request(tgbotapi.NewCallback(cq.ID, "Уже завершено")); err != nil {
			logger.LogCallbackError(err, cq.ID, cq.From.ID)
		}
		return
	}

	if complete {
		_ = r.store.SetStatus(oid, constants.StatusDone)
		if o, err := r.store.LoadOrder(oid); err == nil {
			r.moveCard(o, r.topicDoneID)
			if _, err := r.bot.Send(tgbotapi.NewMessage(o.ChatID, fmt.Sprintf("Заявка №%d завершена. Спасибо! 🎉", oid))); err != nil {
				logger.LogSendError(err, o.ChatID, "completion_notification")
			}
		}
	} else {
		// "Отклонить завершение": остаёмся в Оплаченных (пересоздаём карточку)
		_ = r.store.SetStatus(oid, constants.StatusPaid)
		if o, err := r.store.LoadOrder(oid); err == nil {
			r.moveCard(o, r.topicPaidID)
			if _, err := r.bot.Send(tgbotapi.NewMessage(o.ChatID, fmt.Sprintf("Заявка №%d: завершение отклонено. Администратор свяжется с вами для доработок.", oid))); err != nil {
				logger.LogSendError(err, o.ChatID, "rejection_notification")
			}
		}
	}

	// серым закрываем исходную клавиатуру
	gray := tgbotapi.NewInlineKeyboardMarkup(tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData("✔ Обработано", "noop"),
	))
	if _, err := r.bot.Send(tgbotapi.NewEditMessageReplyMarkup(cq.Message.Chat.ID, cq.Message.MessageID, gray)); err != nil {
		logger.LogSendError(err, cq.Message.Chat.ID, "edit_reply_markup")
	}
	if _, err := r.bot.Request(tgbotapi.NewCallback(cq.ID, "Готово")); err != nil {
		logger.LogCallbackError(err, cq.ID, cq.From.ID)
	}
}

func (r *Router) handleProfilePageCallback(cq *tgbotapi.CallbackQuery) {
	pageStr := strings.TrimPrefix(cq.Data, "profile_page:")
	page, err := strconv.Atoi(pageStr)
	if err != nil || page < 1 {
		if _, err := r.bot.Request(tgbotapi.NewCallback(cq.ID, "Неверная страница")); err != nil {
			logger.LogCallbackError(err, cq.ID, cq.From.ID)
		}
		return
	}

	// Отвечаем на callback
	if _, err := r.bot.Request(tgbotapi.NewCallback(cq.ID, "")); err != nil {
		logger.LogCallbackError(err, cq.ID, cq.From.ID)
	}

	// Показываем нужную страницу
	r.showProfilePage(cq.Message.Chat.ID, cq.From.ID, page)
}

func (r *Router) handleInlineQuery(iq *tgbotapi.InlineQuery) {
	// Безопасность: разрешаем inline-результаты только администраторам
	if _, ok := r.adminIDs[iq.From.ID]; !ok {
		// Можем молча не отвечать, или вернуть пустой ответ
		resp := tgbotapi.InlineConfig{
			InlineQueryID: iq.ID,
			IsPersonal:    true,
			CacheTime:     1,
			Results:       []interface{}{},
		}
		_, _ = r.bot.Request(resp)
		return
	}

	q := strings.TrimSpace(iq.Query)
	if q == "" {
		// Пустой — ничего не показываем
		resp := tgbotapi.InlineConfig{InlineQueryID: iq.ID, IsPersonal: true, CacheTime: 1, Results: []interface{}{}}
		_, _ = r.bot.Request(resp)
		return
	}

	orders, err := r.store.FindOrders(q, 25)
	if err != nil {
		logger.LogRequestError(err, "inline_find", "q", q)
		return
	}

	var results []interface{}
	for idx, o := range orders {
		title := fmt.Sprintf("№%d — %s (%s)", o.ID, o.Service, constants.HumanStatus(o.Status))
		desc := fmt.Sprintf("Дедлайн: %s", o.DeadlineRaw)
		content := tgbotapi.InputTextMessageContent{
			Text: fmt.Sprintf("№%d — %s\nДедлайн: %s\nСтатус: %s\n\ntg://user?id=%d",
				o.ID, o.Service, o.DeadlineRaw, constants.HumanStatus(o.Status), o.UserID),
			ParseMode: "HTML",
		}
		art := tgbotapi.NewInlineQueryResultArticleMarkdown(fmt.Sprintf("o_%d_%d", o.ID, idx), title, content.Text)
		art.Description = desc
		// Кнопки: чат/статус
		art.ReplyMarkup = &tgbotapi.InlineKeyboardMarkup{
			InlineKeyboard: [][]tgbotapi.InlineKeyboardButton{
				{
					tgbotapi.NewInlineKeyboardButtonURL("💬 Открыть чат", fmt.Sprintf("tg://user?id=%d", o.UserID)),
					tgbotapi.NewInlineKeyboardButtonData("🔎 Статус", fmt.Sprintf("stat:%d", o.ID)),
				},
			},
		}
		results = append(results, art)
	}

	resp := tgbotapi.InlineConfig{
		InlineQueryID: iq.ID,
		IsPersonal:    true,
		CacheTime:     1,
		Results:       results,
	}
	if _, err := r.bot.Request(resp); err != nil {
		logger.LogRequestError(err, "inline_answer")
	}
}
