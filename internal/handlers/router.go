package router

import (
	"fmt"
	"log"
	"strconv"
	"strings"
	"sync"

	"tgbot/internal/constants"
	"tgbot/internal/logger"
	"tgbot/internal/parsing"
	"tgbot/internal/ratelimit"
	"tgbot/internal/state"
	"tgbot/internal/storage"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type Router struct {
	bot               *tgbotapi.BotAPI
	started           *state.StartTracker
	sessions          *state.Sessions
	channelID         int64
	channelURL        string
	allowAdminsBypass bool
	adminIDs          map[int64]struct{}
	store             *storage.Store
	boardChatID       int64
	topicInProgressID int
	topicPaidID       int
	topicDoneID       int
	pendingAmount     map[int64]pendingPay
	payLock           sync.Mutex
	deadlineTopicID   int
	faqURL            string
	rateLimiter       *ratelimit.Manager
}

type pendingPay struct {
	OrderID int64
	ChatID  int64
	MsgID   int
}

const (
	cbInstVuz         = "form_inst_vuz"
	cbInstCollege     = "form_inst_college"
	cbInstOther       = "form_inst_other"
	cbCatMain         = "form_cat_main"
	cbCatExtra        = "form_cat_extra"
	cbBackInstitution = "form_back_institution"
	cbBackCategories  = "form_back_categories"
	cbBackWorkType    = "form_back_worktype"
	cbBackDeadline    = "form_back_deadline"
	cbBackPages       = "form_back_pages"
	cbBackTopic       = "form_back_topic"
	cbBackDocs        = "form_back_docs"
	cbDeadlineUnknown = "form_deadline_unknown"
	cbPagesUnknown    = "form_pages_unknown"
	cbDocsReady       = "form_docs_ready"
	cbConfirmSend     = "form_confirm_send"
	cbConfirmEdit     = "form_confirm_edit"
	cbConfirmRestart  = "form_confirm_restart"
)

const (
	cbPrefixWorkMain  = "form_work_main_"
	cbPrefixWorkExtra = "form_work_extra_"
)

var (
	mainWorkTypes  = constants.MainWorkTypes
	extraWorkTypes = constants.ExtraWorkTypes
)

func NewRouter(
	bot *tgbotapi.BotAPI,
	started *state.StartTracker,
	sessions *state.Sessions,
	channelID int64,
	channelURL string,
	allowAdminsBypass bool,
	adminIDsCSV string,
	store *storage.Store,
	boardChatID int64,
	topicInProgressID, topicPaidID, topicDoneID, deadlineTopicID int, faqURL string,
) *Router {
	admins := map[int64]struct{}{}
	for _, p := range strings.Split(adminIDsCSV, ",") {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		if id, err := strconv.ParseInt(p, 10, 64); err == nil {
			admins[id] = struct{}{}
		}
	}
	return &Router{
		bot:               bot,
		started:           started,
		sessions:          sessions,
		channelID:         channelID,
		channelURL:        channelURL,
		allowAdminsBypass: allowAdminsBypass,
		adminIDs:          admins,
		store:             store,
		boardChatID:       boardChatID,
		topicInProgressID: topicInProgressID,
		topicPaidID:       topicPaidID,
		topicDoneID:       topicDoneID,
		deadlineTopicID:   deadlineTopicID,
		faqURL:            faqURL,

		pendingAmount: make(map[int64]pendingPay),
		rateLimiter:   ratelimit.NewManager(ratelimit.GetDefaultLimits()),
	}
}

/* ===================== Обработка апдейтов ===================== */

func (r *Router) Handle(upd tgbotapi.Update) {
	// Проверяем rate limiting для всех типов обновлений
	if !r.checkRateLimit(upd) {
		return
	}

	// 1) Сначала обработаем callback_query (они не содержат Message!)
	if upd.CallbackQuery != nil {
		r.handleCallback(upd.CallbackQuery)
		return
	}

	if upd.InlineQuery != nil {
		r.handleInlineQuery(upd.InlineQuery)
		return
	}

	// 2) Дальше — обычные сообщения
	if upd.ChannelPost != nil {
		log.Printf("channel post: title=%q chat_id=%d", upd.ChannelPost.Chat.Title, upd.ChannelPost.Chat.ID)
		return
	}

	if upd.Message == nil {
		return
	}
	msg := upd.Message
	chatID := msg.Chat.ID

	// ===== ФИЛЬТР ПО ТИПУ ЧАТА =====
	// Работать только в личке; в группах/супергруппах — молчать,
	// кроме борды (boardChatID) — пригодится для модерации.
	if msg.Chat.Type != "private" {
		if chatID == r.boardChatID {
			// Обработка ввода суммы платежа для админов в рабочем чате (включая темы)
			if r.isAdmin(msg.From.ID) && msg.Text != "" {
				log.Printf("Admin %d sent message in board chat: %q", msg.From.ID, msg.Text)
				// Проверяем, является ли это числом (суммой платежа)
				if _, err := strconv.ParseFloat(msg.Text, 64); err == nil {
					log.Printf("Processing payment amount input: %q", msg.Text)
					r.handlePaymentAmountInput(msg)
					return
				}
			}

			// Остальные сообщения игнорируем
			log.Printf("ignore message in board chat %d: %q", chatID, msg.Text)
			return
		}
		// Любые другие группы/каналы — игнор
		return
	}

	// Команды
	if msg.IsCommand() {
		r.handleCommand(msg)
		return
	}

	// До старта
	if !r.started.IsStarted(chatID) {
		r.replyText(chatID, "Нажмите /start, чтобы оформить заявку.")
		return
	}

	// FSM
	sess := r.sessions.Get(chatID)

	// Гейт подписки
	if sess.Stage == state.StageWelcome {
		if msg.Text == "Проверить подписку ✅" {
			if r.isSubscribed(msg.From.ID) {
				ok := tgbotapi.NewMessage(chatID, "Спасибо за подписку! Продолжаем.")
				ok.ReplyMarkup = tgbotapi.NewRemoveKeyboard(true)
				if _, err := r.bot.Send(ok); err != nil {
					logger.LogSendError(err, chatID, "subscription_confirmation")
				}
				r.startQuestionnaire(chatID, sess)
				return
			}
			r.replyText(chatID, "Похоже, подписки пока нет. Подпишитесь и нажмите «Проверить подписку ✅».")
			r.sendGate(chatID)
			return
		}
		r.sendGate(chatID)
		return
	}

	// Если ждём файл чека — обрабатываем тут
	if sess.Stage == state.StageAwaitReceiptUpload {
		r.handleReceiptUpload(msg, sess)
		return
	}

	// Текст/файлы не по делу
	if msg.Text == "" && msg.Document == nil && len(msg.Photo) == 0 {
		r.replyText(chatID, "Пожалуйста, напишите текстовое сообщение или прикрепите документ.")
		return
	}

	switch sess.Stage {

	case state.StageChooseInstitution:
		r.replyText(chatID, "Выберите тип учреждения через кнопки под сообщением.")
		return

	case state.StageAskDeadline:
		if msg.Text == "" {
			// Показываем кнопки если их нет
			kb := r.createDeadlineKeyboard()
			m := tgbotapi.NewMessage(chatID, "Укажите дедлайн текстом (дата или промежуток) или выберите кнопку.")
			m.ReplyMarkup = kb
			r.bot.Send(m)
			return
		}

		// ↓ вместо ValidateDeadlineInput — сразу полноценный парс
		res := parsing.ParseDeadline(msg.Text)
		if !res.IsValid {
			// Показываем кнопки при ошибке
			kb := r.createDeadlineKeyboard()
			m := tgbotapi.NewMessage(chatID, fmt.Sprintf("❌ %s\n\nПопробуйте ещё раз или выберите кнопку", res.Error))
			m.ReplyMarkup = kb
			r.bot.Send(m)
			return
		}

		// Сохраняем нормализованное значение для показа везде дальше
		// Если это относительный срок — тут уже будет дата в формате 02.01.2006
		// Если «не знаю» — будет "не определён"
		sess.Deadline = res.HumanFormat

		r.navigateToPages(chatID)
		return

	case state.StageChooseWorkCategory:
		r.replyText(chatID, "Выбирайте категорию через кнопки под сообщением.")
		return

	case state.StageChooseWorkType:
		r.replyText(chatID, "Выберите тип услуги через кнопки под сообщением.")
		return

	case state.StageAskPages:
		if p, ok := normalizePages(msg.Text); ok {
			sess.Pages = p
		} else {
			// Показываем кнопку "Назад" при ошибке
			kb := r.createPagesKeyboard()
			m := tgbotapi.NewMessage(chatID, "Не понял формат. Введите число (например, 30) или диапазон (20–40), либо нажмите «Пока не определился».")
			m.ReplyMarkup = kb
			r.bot.Send(m)
			return
		}
		r.navigateToTopic(chatID)
		return

	case state.StageAskTopic:
		// Сохраняем тему
		sess.Topic = msg.Text

		// Автоматически переходим к требованиям
		sess.Stage = state.StageAskRequirements
		r.navigateToRequirements(chatID)
		return

	case state.StageAskRequirements:
		// Сохраняем требования
		sess.Requirements = msg.Text

		// Автоматически переходим к этапу загрузки документов
		r.navigateToDocs(chatID)
		return

	case state.StageAskDocs:
		collected := false
		if msg.Document != nil {
			// Валидация размера файла (20 МБ = 20 * 1024 * 1024 байт)
			const maxFileSize = 20 * 1024 * 1024
			if msg.Document.FileSize > maxFileSize {
				r.replyText(chatID, "❌ Файл слишком большой. Максимальный размер: 20 МБ.")
				return
			}

			// Валидация типа файла
			allowedTypes := map[string]bool{
				"application/pdf":    true,
				"image/jpeg":         true,
				"image/jpg":          true,
				"image/png":          true,
				"application/msword": true,
				"application/vnd.openxmlformats-officedocument.wordprocessingml.document": true,
			}
			if !allowedTypes[msg.Document.MimeType] {
				r.replyText(chatID, "❌ Неподдерживаемый тип файла. Разрешены: PDF, JPG, PNG, DOC, DOCX.")
				return
			}

			sess.AttachIDs = append(sess.AttachIDs, msg.Document.FileID)
			collected = true
		}
		if photos := msg.Photo; len(photos) > 0 {
			sess.AttachIDs = append(sess.AttachIDs, photos[len(photos)-1].FileID)
			collected = true
		}
		if collected {
			r.replyText(chatID, "Документ(ы) принял. Нажмите «Готово, документов больше нет», когда закончите.")
			return
		}
		if msg.Text != "" {
			r.replyText(chatID, "Чтобы завершить этап, используйте кнопку «Готово, документов больше нет».")
		}
		return

	case state.StageAskClientSource:
		if strings.TrimSpace(msg.Text) == "" {
			r.replyText(chatID, "Укажите источник (например, имя, ссылка или ник).")
			return
		}
		sess.ClientSource = msg.Text
		r.sendSummary(chatID)
		return

	case state.StageConfirm:
		switch msg.Text {
		case "✅ Отправить":
			r.handleConfirmSendAction(chatID, msg.From.ID, sess)
			return
		case "✏️ Изменить":
			r.handleConfirmEditAction(chatID, sess)
			return

		case "🔄 Заполнить заново":
			r.handleConfirmRestartAction(chatID)
			return

		default:
			r.replyText(chatID, "Выберите действие с помощью кнопок под сообщением.")
			return
		}
	}

	// Обработка ввода суммы платежа (только для админов, когда не в FSM)
	if r.isAdmin(msg.From.ID) && msg.Text != "" {
		// Проверяем, является ли это числом (суммой платежа)
		if _, err := strconv.ParseFloat(msg.Text, 64); err == nil {
			r.handlePaymentAmountInput(msg)
			return
		}
	}
}

// checkRateLimit проверяет rate limiting для обновления
func (r *Router) checkRateLimit(upd tgbotapi.Update) bool {
	var userID int64
	var actionType ratelimit.ActionType

	// Определяем тип действия и пользователя
	switch {
	case upd.CallbackQuery != nil:
		userID = upd.CallbackQuery.From.ID
		actionType = ratelimit.ActionCallback

	case upd.Message != nil:
		userID = upd.Message.From.ID

		// Определяем подтип сообщения
		if upd.Message.Document != nil || len(upd.Message.Photo) > 0 {
			actionType = ratelimit.ActionFileUpload
		} else if strings.HasPrefix(upd.Message.Text, "/") {
			actionType = ratelimit.ActionCommand
		} else {
			actionType = ratelimit.ActionMessage
		}

	case upd.InlineQuery != nil:
		userID = upd.InlineQuery.From.ID
		actionType = ratelimit.ActionMessage

	default:
		return true // Неизвестный тип обновления - разрешаем
	}

	// Админы не ограничены
	if r.isAdmin(userID) {
		return true
	}

	// Проверяем лимиты
	if !r.rateLimiter.IsAllowed(userID, actionType) {
		// Отправляем сообщение о превышении лимита
		r.handleRateLimitExceeded(userID, actionType)
		return false
	}

	return true
}

// handleRateLimitExceeded обрабатывает превышение лимитов
func (r *Router) handleRateLimitExceeded(userID int64, actionType ratelimit.ActionType) {
	// Определяем тип сообщения об ошибке
	var message string
	switch actionType {
	case ratelimit.ActionMessage:
		message = "⏳ Слишком много сообщений. Подождите минуту и попробуйте снова."
	case ratelimit.ActionFileUpload:
		message = "⏳ Слишком много файлов. Подождите 2 минуты и попробуйте снова."
	case ratelimit.ActionCallback:
		message = "⏳ Слишком много действий. Подождите 30 секунд и попробуйте снова."
	case ratelimit.ActionCommand:
		message = "⏳ Слишком много команд. Подождите минуту и попробуйте снова."
	default:
		message = "⏳ Слишком много запросов. Подождите минуту и попробуйте снова."
	}

	// Отправляем сообщение пользователю
	msg := tgbotapi.NewMessage(userID, message)
	r.bot.Send(msg)
}

// GetRateLimiter возвращает rate limiter для внешнего использования
func (r *Router) GetRateLimiter() interface{} {
	return r.rateLimiter
}

// ===== КЛАВИАТУРЫ =====

// createInstitutionKeyboard создает inline-клавиатуру для выбора типа учреждения
func (r *Router) createInstitutionKeyboard() tgbotapi.InlineKeyboardMarkup {
	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("Вуз", cbInstVuz),
			tgbotapi.NewInlineKeyboardButtonData("Колледж", cbInstCollege),
			tgbotapi.NewInlineKeyboardButtonData("Другое", cbInstOther),
		),
	)
}

// createCategoryKeyboard создает inline-клавиатуру для выбора категории услуг
func (r *Router) createCategoryKeyboard() tgbotapi.InlineKeyboardMarkup {
	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("📝 Основные услуги", cbCatMain),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("📊 Доп. услуги", cbCatExtra),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("◀️ Назад к выбору заведения", cbBackInstitution),
		),
	)
}

// createWorkTypeKeyboard создает inline-клавиатуру для выбора типа работ в зависимости от категории
func (r *Router) createWorkTypeKeyboard(category string) tgbotapi.InlineKeyboardMarkup {
	var rows [][]tgbotapi.InlineKeyboardButton
	if category == "Основные услуги" {
		rows = append(rows, buildInlineRows(mainWorkTypes, cbPrefixWorkMain)...)
	} else {
		rows = append(rows, buildInlineRows(extraWorkTypes, cbPrefixWorkExtra)...)
	}
	rows = append(rows, []tgbotapi.InlineKeyboardButton{
		tgbotapi.NewInlineKeyboardButtonData("◀️ Назад к категориям", cbBackCategories),
	})
	return tgbotapi.InlineKeyboardMarkup{InlineKeyboard: rows}
}

// createDeadlineKeyboard создает inline-клавиатуру для ввода дедлайна
func (r *Router) createDeadlineKeyboard() tgbotapi.InlineKeyboardMarkup {
	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("Не знаю", cbDeadlineUnknown),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("◀️ Назад к типу работы", cbBackWorkType),
		),
	)
}

// createPagesKeyboard создает inline-клавиатуру для ввода объема страниц
func (r *Router) createPagesKeyboard() tgbotapi.InlineKeyboardMarkup {
	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("Пока не определился", cbPagesUnknown),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("◀️ Назад к дедлайну", cbBackDeadline),
		),
	)
}

// createTopicKeyboard создает inline-клавиатуру для ввода темы
func (r *Router) createTopicKeyboard() tgbotapi.InlineKeyboardMarkup {
	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("◀️ Назад к объёму", cbBackPages),
		),
	)
}

// createRequirementsKeyboard создает inline-клавиатуру для ввода требований
func (r *Router) createRequirementsKeyboard() tgbotapi.InlineKeyboardMarkup {
	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("◀️ Назад к теме", cbBackTopic),
		),
	)
}

// createDocsKeyboard создает inline-клавиатуру для этапа загрузки документов
func (r *Router) createDocsKeyboard() tgbotapi.InlineKeyboardMarkup {
	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("Готово, документов больше нет", cbDocsReady),
		),
	)
}

// createClientSourceKeyboard создает inline-клавиатуру для источника клиента
func (r *Router) createClientSourceKeyboard() tgbotapi.InlineKeyboardMarkup {
	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("◀️ Назад к документам", cbBackDocs),
		),
	)
}

// createConfirmKeyboard создает inline-клавиатуру для подтверждения заявки
func (r *Router) createConfirmKeyboard() tgbotapi.InlineKeyboardMarkup {
	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("✅ Отправить", cbConfirmSend),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("✏️ Изменить", cbConfirmEdit),
			tgbotapi.NewInlineKeyboardButtonData("🔄 Заполнить заново", cbConfirmRestart),
		),
	)
}

func buildInlineRows(options []string, prefix string) [][]tgbotapi.InlineKeyboardButton {
	var rows [][]tgbotapi.InlineKeyboardButton
	var current []tgbotapi.InlineKeyboardButton

	for idx, text := range options {
		btn := tgbotapi.NewInlineKeyboardButtonData(text, prefix+strconv.Itoa(idx))
		current = append(current, btn)

		if len(current) == 2 || idx == len(options)-1 {
			rows = append(rows, current)
			current = nil
		}
	}

	return rows
}

// ===== НАВИГАЦИЯ =====

// navigateToInstitution переходит к выбору типа учреждения
func (r *Router) navigateToInstitution(chatID int64) {
	sess := r.sessions.Get(chatID)
	sess.Stage = state.StageChooseInstitution

	kb := r.createInstitutionKeyboard()
	m := tgbotapi.NewMessage(chatID, "Выберите тип учреждения:")
	m.ReplyMarkup = kb
	r.bot.Send(m)
}

// navigateToCategory переходит к выбору категории работ
func (r *Router) navigateToCategory(chatID int64) {
	sess := r.sessions.Get(chatID)
	sess.Stage = state.StageChooseWorkCategory

	kb := r.createCategoryKeyboard()
	m := tgbotapi.NewMessage(chatID, "Выберите категорию работ:")
	m.ReplyMarkup = kb
	r.bot.Send(m)
}

// navigateToWorkType переходит к выбору типа работ
func (r *Router) navigateToWorkType(chatID int64, category string) {
	sess := r.sessions.Get(chatID)
	sess.WorkCategory = category
	sess.Stage = state.StageChooseWorkType

	kb := r.createWorkTypeKeyboard(category)
	message := "Выберите тип работы:"
	if category == "Доп. Услуги" {
		message = "Выберите дополнительную услугу:"
	}

	m := tgbotapi.NewMessage(chatID, message)
	m.ReplyMarkup = kb
	r.bot.Send(m)
}

// navigateToDeadline переходит к вводу дедлайна
func (r *Router) navigateToDeadline(chatID int64) {
	sess := r.sessions.Get(chatID)
	sess.Stage = state.StageAskDeadline

	kb := r.createDeadlineKeyboard()
	m := tgbotapi.NewMessage(chatID, "Укажите дедлайн: дата (например, 11.11.25) или промежуток (например, 1.5 недели).")
	m.ReplyMarkup = kb
	r.bot.Send(m)
}

// navigateToPages переходит к вводу объема страниц
func (r *Router) navigateToPages(chatID int64) {
	sess := r.sessions.Get(chatID)
	sess.Stage = state.StageAskPages

	kb := r.createPagesKeyboard()
	message := fmt.Sprintf("✅ Дедлайн: %s\n\nУкажите объём в страницах (например, 30 или 20–40). Если не определились — выберите кнопку.", sess.Deadline)

	m := tgbotapi.NewMessage(chatID, message)
	m.ReplyMarkup = kb
	r.bot.Send(m)
}

// navigateToTopic переходит к вводу темы
func (r *Router) navigateToTopic(chatID int64) {
	sess := r.sessions.Get(chatID)
	sess.Stage = state.StageAskTopic

	// Очищаем тему при переходе к её редактированию
	sess.Topic = ""

	kb := r.createTopicKeyboard()
	message := fmt.Sprintf("✅ Объём: %s\n\nНапишите тему работы:", sess.Pages)

	m := tgbotapi.NewMessage(chatID, message)
	m.ReplyMarkup = kb
	r.bot.Send(m)
}

// navigateToRequirements переходит к вводу требований
func (r *Router) navigateToRequirements(chatID int64) {
	sess := r.sessions.Get(chatID)
	sess.Stage = state.StageAskRequirements

	// Очищаем требования при переходе к их редактированию
	sess.Requirements = ""

	kb := r.createRequirementsKeyboard()
	message := fmt.Sprintf("✅ Описание: %s\n\nНапишите дополнительную информацию:", sess.Topic)

	m := tgbotapi.NewMessage(chatID, message)
	m.ReplyMarkup = kb
	r.bot.Send(m)
}

// navigateToDocs переходит к этапу загрузки документов
func (r *Router) navigateToDocs(chatID int64) {
	sess := r.sessions.Get(chatID)
	sess.Stage = state.StageAskDocs

	m := tgbotapi.NewMessage(chatID, "Прикрепите документы (техническое задание, инструкции, дополнительные материалы).\n\nКогда всё приложите, нажмите кнопку «Готово, документов больше нет» под сообщением.\nЕсли файлов нет, просто нажмите эту кнопку.")
	m.ReplyMarkup = r.createDocsKeyboard()
	r.bot.Send(m)
}

// navigateToClientSource переходит к вводу источника клиента
func (r *Router) navigateToClientSource(chatID int64) {
	sess := r.sessions.Get(chatID)
	sess.Stage = state.StageAskClientSource
	sess.ClientSource = ""

	kb := r.createClientSourceKeyboard()
	m := tgbotapi.NewMessage(chatID, "Укажите источник (имя, ссылка или ник). Это поможет нам отслеживать откуда приходят заявки.")
	m.ReplyMarkup = kb
	r.bot.Send(m)
}

func (r *Router) clearInlineKeyboard(chatID int64, messageID int) {
	edit := tgbotapi.NewEditMessageReplyMarkup(chatID, messageID, tgbotapi.InlineKeyboardMarkup{})
	if _, err := r.bot.Send(edit); err != nil {
		logger.LogSendError(err, chatID, "clear_inline_keyboard")
	}
}

func (r *Router) sendSummary(chatID int64) {
	sess := r.sessions.Get(chatID)
	summary := r.buildSummary(sess)
	m := tgbotapi.NewMessage(chatID, summary)
	m.ReplyMarkup = r.createConfirmKeyboard()
	if _, err := r.bot.Send(m); err != nil {
		logger.LogSendError(err, chatID, "order_summary")
		return
	}
	sess.Stage = state.StageConfirm
}

func (r *Router) handleConfirmSendAction(chatID, userID int64, sess *state.Session) {
	source := strings.TrimSpace(sess.ClientSource)
	if source == "" {
		source = "Не указано"
	}
	order := &storage.Order{
		UserID:       userID,
		ChatID:       chatID,
		Service:      sess.WorkType,
		DeadlineRaw:  sess.Deadline,
		Pages:        sess.Pages,
		Topic:        sess.Topic,
		Requirements: sess.Requirements,
		ClientSource: source,
		Status:       constants.StatusAwaitPay,
	}
	id, err := r.store.CreateOrder(order)
	if err != nil {
		logger.LogOrderError(err, 0, "create_order")
		r.replyText(chatID, "Не удалось сохранить заявку. Попробуйте ещё раз.")
		return
	}

	if o, err := r.store.LoadOrder(id); err == nil {
		r.moveCard(o, r.topicInProgressID)
		for _, fid := range sess.AttachIDs {
			if _, err := r.sendDocumentToThread(r.boardChatID, r.topicInProgressID, fid, "", nil); err != nil {
				logger.LogBoardError(err, id, "send_attachment", r.topicInProgressID)
			}
		}
	}

	done := tgbotapi.NewMessage(chatID, fmt.Sprintf("Заявка №%d отправлена. Для оплаты и загрузки подтверждения используйте /profile", id))
	done.ReplyMarkup = tgbotapi.NewRemoveKeyboard(true)
	if _, err := r.bot.Send(done); err != nil {
		logger.LogSendError(err, chatID, "order_confirmation")
	}

	r.sessions.Reset(chatID)
}

func (r *Router) handleConfirmEditAction(chatID int64, sess *state.Session) {
	sess.Topic = ""
	sess.Requirements = ""
	sess.ClientSource = ""
	r.navigateToTopic(chatID)
}

func (r *Router) handleConfirmRestartAction(chatID int64) {
	r.sessions.Reset(chatID)
	r.startQuestionnaire(chatID, r.sessions.Get(chatID))
}
