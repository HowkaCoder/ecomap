package telegram

import (
	"context"
	"errors"
	"fmt"
	"html"
	"log"
	"strconv"
	"strings"
	"sync"
	"time"
	_ "time/tzdata"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"ecomap/internal/domain/bin"
	"ecomap/internal/dto"
	"ecomap/internal/usecase"
)

type step int

const (
	stepNone step = iota
	stepFullName
	stepBinPhoto
	stepBinAddress
	stepBinLocation
	stepDumpPhoto
	stepDumpLocation
	stepDumpAddress
	stepDumpLandmark
	stepDumpDesc
	stepDumpSeverity
	stepPickupWeight
	stepIssueDescription
)

const (
	btnRegister  = "🧍 Úydi qosıw"
	btnDump      = "🗑 Qoqıs haqqında xabar"
	btnHelp      = "ℹ️ Kómek"
	btnMyBins    = "♻️ Meniń konteynerlerim"
	btnSchedule  = "🚚 Alıp ketiw kestesi"
	btnMissed    = "⚠️ Alıp ketpedi"
	btnImpact    = "🌱 Eko-balans"
	btnSort      = "🧭 Qalay sortlaw?"
	btnIssues    = "📋 Meniń múrájatlarım"
	btnReminders = "🔔 Esletpeler"
	btnLanguage  = "🗣 Til / Язык"
	btnMiniApp   = "🌿 EcoMap Mini App"
	btnStats     = "📊 Statistika"
)

type Config struct {
	Admins       string
	Collectors   string
	Recyclers    string
	Schedule     string
	Timezone     string
	ReminderHour int
	MiniAppURL string
}

type session struct {
	step       step
	user       dto.RegisterUserInput
	dump       dto.DumpReportInput
	issueBinID uint64
	pickupQR   string
}

type Handler struct {
	bot        *tgbotapi.BotAPI
	registerUC *usecase.RegisterUserUseCase
	dumpUC     *usecase.DumpReportUsecase
	workflowUC *usecase.WorkflowUseCase

	admins     map[int64]bool
	collectors map[int64]bool
	recyclers  map[int64]bool
	schedule   map[time.Weekday][]string
	location   *time.Location
	reminderHour int
	miniAppURL string
	languages  map[int64]string

	mu       sync.Mutex
	sessions map[int64]*session
}

func New(bot *tgbotapi.BotAPI, registerUC *usecase.RegisterUserUseCase, dumpUC *usecase.DumpReportUsecase, workflowUC *usecase.WorkflowUseCase, cfg Config) *Handler {
	location, err := time.LoadLocation(cfg.Timezone)
	if err != nil {
		location = time.FixedZone("Asia/Tashkent", 5*60*60)
	}
	if cfg.ReminderHour < 0 || cfg.ReminderHour > 23 {
		cfg.ReminderHour = 19
	}
	h := &Handler{
		bot: bot, registerUC: registerUC, dumpUC: dumpUC, workflowUC: workflowUC,
		admins: parseIDs(cfg.Admins), collectors: parseIDs(cfg.Collectors), recyclers: parseIDs(cfg.Recyclers),
		schedule: parseSchedule(cfg.Schedule), location: location, reminderHour: cfg.ReminderHour, miniAppURL: cfg.MiniAppURL,
		sessions: make(map[int64]*session), languages: make(map[int64]string),
	}
	return h
}

func (h *Handler) Start(ctx context.Context) {
	go h.reminderLoop(ctx)
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60
	updates := h.bot.GetUpdatesChan(u)
	for {
		select {
		case <-ctx.Done():
			h.bot.StopReceivingUpdates()
			return
		case update, ok := <-updates:
			if !ok {
				return
			}
			h.handleUpdate(ctx, update)
		}
	}
}

func (h *Handler) handleUpdate(ctx context.Context, update tgbotapi.Update) {
	if update.Message != nil && update.Message.From != nil {
		h.loadLanguage(ctx, update.Message.From.ID)
	}
	if update.CallbackQuery != nil {
		h.loadLanguage(ctx, update.CallbackQuery.From.ID)
	}
	switch {
	case update.CallbackQuery != nil:
		h.handleCallback(ctx, update.CallbackQuery)
	case update.Message == nil:
		return
	case update.Message.IsCommand():
		h.handleCommand(ctx, update.Message)
	default:
		h.handleMessage(ctx, update.Message)
	}
}

func (h *Handler) send(c tgbotapi.Chattable) {
	if _, err := h.bot.Send(c); err != nil {
		log.Printf("telegram send: %v", err)
	}
}

func (h *Handler) reply(chatID int64, text string) {
	m := tgbotapi.NewMessage(chatID, h.localize(chatID, text))
	m.ParseMode = tgbotapi.ModeHTML
	m.ReplyMarkup = mainMenu(h.language(chatID))
	h.send(m)
}

func (h *Handler) handleCommand(ctx context.Context, msg *tgbotapi.Message) {
	parts := strings.Fields(msg.Text)
	if len(parts) == 0 {
		return
	}
	args := parts[1:]
	switch msg.Command() {
	case "start":
		h.dropSession(msg.Chat.ID)
		if len(args) > 0 && strings.HasPrefix(args[0], "pickup_") {
			h.beginPickup(ctx, msg.Chat.ID, msg.From.ID, strings.TrimPrefix(args[0], "pickup_"))
			return
		}
		h.sendWelcome(msg.Chat.ID, msg.From.ID)
	case "help":
		h.showHelp(msg.Chat.ID)
	case "language":
		h.showLanguage(msg.Chat.ID)
	case "id":
		h.reply(msg.Chat.ID, fmt.Sprintf("Ваш Telegram ID: <code>%d</code>", msg.From.ID))
	case "bins":
		h.showUserBins(ctx, msg.Chat.ID, msg.From.ID)
	case "calendar":
		h.showSchedule(msg.Chat.ID)
	case "missed":
		h.startMissedPickup(ctx, msg.Chat.ID, msg.From.ID)
	case "sort":
		h.showSortingGuide(msg.Chat.ID)
	case "impact":
		h.showImpact(ctx, msg.Chat.ID, msg.From.ID)
	case "myreports", "reports":
		h.showUserReports(ctx, msg.Chat.ID, msg.From.ID)
	case "reminders":
		h.handleReminders(ctx, msg.Chat.ID, msg.From.ID, args)
	case "batch":
		h.startBatch(ctx, msg.Chat.ID, msg.From.ID)
	case "finishbatch":
		h.finishBatch(ctx, msg.Chat.ID, msg.From.ID)
	case "accept":
		h.acceptBatch(ctx, msg.Chat.ID, msg.From.ID, args)
	case "issues":
		h.showOpenIssues(ctx, msg.Chat.ID, msg.From.ID)
	case "dumps":
		h.showOpenDumps(ctx, msg.Chat.ID, msg.From.ID)
	case "resolve_issue":
		h.resolveIssue(ctx, msg.Chat.ID, msg.From.ID, args)
	case "resolve_dump":
		h.resolveDump(ctx, msg.Chat.ID, msg.From.ID, args)
	case "admin":
		h.showAdminHelp(msg.Chat.ID, msg.From.ID)
	case "stats":
		h.showAdminStats(msg.Chat.ID, msg.From.ID)
	default:
		h.reply(msg.Chat.ID, "Не понял команду. Откройте /help.")
	}
}

func (h *Handler) sendWelcome(chatID, userID int64) {
	text := "🌿 <b>EcoMap · Bin4Home</b>\n\nPlastik, qaǵaz hám shiyshe bólek jıynaladı. Alıp ketiw kestesi, múrájatlar hám qayta islewge tapsırılǵan salmaq — bári bir jerde."
	if h.language(userID) == "ru" {
		text = "🌿 <b>EcoMap · Bin4Home</b>\n\nРаздельный сбор пластика, бумаги и стекла. График вывоза, обращения и подтверждённый вес переработки — в одном месте."
	}
	m := tgbotapi.NewMessage(chatID, text)
	m.ParseMode = tgbotapi.ModeHTML
	m.ReplyMarkup = mainMenu(h.language(userID))
	h.send(m)
	if h.miniAppURL != "" {
		rows := [][]tgbotapi.InlineKeyboardButton{{{
			Text: btnMiniApp,
			WebApp: &tgbotapi.WebAppInfo{URL: h.miniAppURL},
		}}}
		if h.isAdmin(userID) {
			rows = append(rows, tgbotapi.NewInlineKeyboardRow(tgbotapi.InlineKeyboardButton{
				Text: btnStats,
				WebApp: &tgbotapi.WebAppInfo{URL: h.miniAppURL + "?page=admin"},
			}))
		}
		appMessage := tgbotapi.NewMessage(chatID, "🌿 EcoMap Mini App · úy konteynerleri, kestesi hám eko-balans.")
		appMessage.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(rows...)
		h.send(appMessage)
	}
}

func (h *Handler) showAdminStats(chatID, userID int64) {
	if !h.isAdmin(userID) {
		h.reply(chatID, "Bul bet tek administrator ushın.")
		return
	}
	if h.miniAppURL == "" {
		h.reply(chatID, "Mini App URL sazlanbaǵan.")
		return
	}
	m := tgbotapi.NewMessage(chatID, "📊 Tolıq statistika hám operaciyalıq kórsetkishler:")
	m.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(tgbotapi.NewInlineKeyboardRow(tgbotapi.InlineKeyboardButton{
		Text: btnStats,
		WebApp: &tgbotapi.WebAppInfo{URL: h.miniAppURL + "?page=admin"},
	}))
	h.send(m)
}

func (h *Handler) showHelp(chatID int64) {
 h.reply(chatID, "🌱 <b>EcoMap</b>\n\n♻️ Контейнеры — /bins\n🚚 График — /calendar\n⚠️ Пропущенный вывоз — /missed\n🗑 Сообщить о свалке — кнопка в меню\n🧭 Сортировка — /sort\n📋 Мои обращения — /myreports\n🌿 Подтверждённый вес — /impact\n🔔 Напоминания — /reminders\n🆔 Мой ID — /id\n\nСборщикам: /batch, затем сканируйте QR контейнера камерой телефона и введите вес в боте. Завершение рейса: /finishbatch.")
}

func (h *Handler) handleMessage(ctx context.Context, msg *tgbotapi.Message) {
	chatID := msg.Chat.ID
	switch msg.Text {
	case btnMyBins, "♻️ Мои контейнеры":
		h.dropSession(chatID)
		h.showUserBins(ctx, chatID, msg.From.ID)
		return
	case btnDump, "🗑 Сообщить о свалке":
		h.startDumpReport(chatID, msg.From.ID)
		return
	case btnHelp, "ℹ️ Помощь":
		h.dropSession(chatID)
		h.showHelp(chatID)
		return
	case btnSchedule, "🚚 График вывоза":
		h.showSchedule(chatID)
		return
	case btnMissed, "⚠️ Не забрали":
		h.startMissedPickup(ctx, chatID, msg.From.ID)
		return
	case btnImpact, "🌱 Экобаланс":
		h.showImpact(ctx, chatID, msg.From.ID)
		return
	case btnSort, "🧭 Что куда?":
		h.showSortingGuide(chatID)
		return
	case btnIssues, "📋 Мои обращения":
		h.showUserReports(ctx, chatID, msg.From.ID)
		return
	case btnReminders, "🔔 Напоминания":
		h.handleReminders(ctx, chatID, msg.From.ID, nil)
		return
	case btnLanguage, "🗣 Язык / Til":
		h.showLanguage(chatID)
		return
	case btnRegister, "🧍 Регистрация":
		h.startRegistration(chatID, msg.From.ID)
		return
	}

	h.mu.Lock()
	sess, ok := h.sessions[chatID]
	h.mu.Unlock()
	if !ok {
		m := h.newMessage(chatID, "Нажмите /start, чтобы открыть меню.")
		m.ReplyMarkup = mainMenu(h.language(chatID))
		h.send(m)
		return
	}

	switch sess.step {
	case stepDumpPhoto:
		if len(msg.Photo) == 0 { h.reply(chatID, "📸 Отправьте фотографию свалки."); return }
		sess.dump.PhotoID = msg.Photo[len(msg.Photo)-1].FileID
		sess.step = stepDumpLocation
		h.reply(chatID, "📍 Отправьте геолокацию свалки.")
	case stepDumpLocation:
		if msg.Location == nil { h.reply(chatID, "📍 Нужна геолокация. Нажмите скрепку → Геопозиция."); return }
		sess.dump.Lat, sess.dump.Lon = msg.Location.Latitude, msg.Location.Longitude
		sess.step = stepDumpAddress
		h.reply(chatID, "🏠 Укажите адрес, если знаете его.")
	case stepDumpAddress:
		sess.dump.Address = msg.Text
		sess.step = stepDumpLandmark
		h.reply(chatID, "🧭 Добавьте ориентир: школу, магазин или перекрёсток.")
	case stepDumpLandmark:
		sess.dump.Landmark = msg.Text
		sess.step = stepDumpDesc
		h.reply(chatID, "📝 Кратко опишите проблему.")
	case stepDumpDesc:
		sess.dump.Description = msg.Text
		sess.step = stepDumpSeverity
		h.reply(chatID, "🚨 Укажите срочность: обычная, опасная или срочно.")
	case stepDumpSeverity:
		sess.dump.Status = msg.Text
		id, err := h.dumpUC.Execute(ctx, sess.dump)
		if errors.Is(err, usecase.ErrNotRegistered) { h.finish(chatID, "Сначала зарегистрируйтесь через меню."); return }
		if err != nil { log.Printf("dump report: %v", err); h.reply(chatID, "Не удалось сохранить обращение. Попробуйте ещё раз."); return }
		h.finish(chatID, fmt.Sprintf("✅ Обращение #%d принято. Статус можно проверить в разделе «Мои обращения».", id))
	case stepFullName:
		if strings.TrimSpace(msg.Text) == "" { h.reply(chatID, "Введите имя текстом."); return }
		sess.user.FullName = strings.TrimSpace(msg.Text)
		sess.step = stepBinPhoto
		h.reply(chatID, "📸 Отправьте фото одного из контейнеров.")
	case stepBinPhoto:
		if len(msg.Photo) == 0 { h.reply(chatID, "📸 Нужна фотография контейнера."); return }
		sess.user.BinPhoto = msg.Photo[len(msg.Photo)-1].FileID
		sess.step = stepBinAddress
		h.reply(chatID, "🏠 Укажите адрес дома или точки сбора.")
	case stepBinAddress:
		sess.user.Address = strings.TrimSpace(msg.Text)
		sess.step = stepBinLocation
		location := h.newMessage(chatID, "📍 Отправьте геолокацию точки сбора.")
		keyboard := tgbotapi.NewReplyKeyboard(tgbotapi.NewKeyboardButtonRow(tgbotapi.NewKeyboardButtonLocation("📍 Поделиться геолокацией")))
		keyboard.ResizeKeyboard = true
		location.ReplyMarkup = keyboard
		h.send(location)
	case stepBinLocation:
		if msg.Location == nil { h.reply(chatID, "📍 Нужна геолокация. Нажмите скрепку → Геопозиция."); return }
		sess.user.Lat, sess.user.Lon = msg.Location.Latitude, msg.Location.Longitude
		h.finishRegistration(ctx, chatID, sess)
	case stepPickupWeight:
		weight, err := strconv.ParseFloat(strings.ReplaceAll(strings.TrimSpace(msg.Text), ",", "."), 64)
		if err != nil { h.reply(chatID, "Введите вес числом в килограммах, например 8.5."); return }
		batch, ownerID, err := h.workflowUC.RecordPickup(ctx, msg.From.ID, sess.pickupQR, weight)
		if errors.Is(err, usecase.ErrNoOpenBatch) { h.finish(chatID, "Сначала откройте рейс командой /batch."); return }
		if errors.Is(err, usecase.ErrInvalidWeight) { h.reply(chatID, "Вес должен быть больше 0 и не более 1 000 кг."); return }
		if errors.Is(err, usecase.ErrMaterialMismatch) { h.reply(chatID, "Этот QR относится к другой фракции. Создайте рейс нужного типа через /batch."); return }
		if errors.Is(err, usecase.ErrBinNotFound) { h.finish(chatID, "QR-код контейнера не найден."); return }
		if err != nil { h.reply(chatID, "Не удалось записать вывоз: "+err.Error()); return }
		if ownerID != msg.From.ID {
			h.reply(ownerID, fmt.Sprintf("♻️ Контейнер забран. В рейс %s внесено %.2f кг.", batch.Code, weight))
		}
		h.finish(chatID, fmt.Sprintf("✅ Забор записан · рейс %s · %.2f кг · всего %.2f кг.", batch.Code, weight, batch.CollectedKG))
	case stepIssueDescription:
		id, err := h.workflowUC.CreateIssue(ctx, msg.From.ID, sess.issueBinID, msg.Text)
		if err != nil { h.reply(chatID, "Не удалось создать обращение. Проверьте контейнер и попробуйте снова."); return }
		h.finish(chatID, fmt.Sprintf("✅ Заявка о пропущенном вывозе #%d создана. Статус — /myreports.", id))
	default:
		h.reply(chatID, "Выберите действие в меню.")
	}
}

func (h *Handler) handleCallback(ctx context.Context, cb *tgbotapi.CallbackQuery) {
	if _, err := h.bot.Request(tgbotapi.NewCallback(cb.ID, "")); err != nil { log.Printf("answer callback: %v", err) }
	if cb.Message == nil { return }
	chatID, userID, data := cb.Message.Chat.ID, cb.From.ID, cb.Data

	switch {
	case data == "menu_bins":
		h.showUserBins(ctx, chatID, userID)
	case data == "menu_schedule":
		h.showSchedule(chatID)
	case data == "menu_missed":
		h.startMissedPickup(ctx, chatID, userID)
	case data == "menu_dump":
		h.startDumpReport(chatID, userID)
	case data == "menu_impact":
		h.showImpact(ctx, chatID, userID)
	case data == "menu_sort":
		h.showSortingGuide(chatID)
	case data == "menu_reports":
		h.showUserReports(ctx, chatID, userID)
	case data == "menu_reminders":
		h.handleReminders(ctx, chatID, userID, nil)
	case data == "language_kaa" || data == "language_ru":
		language := strings.TrimPrefix(data, "language_")
		if err := h.workflowUC.SetLanguage(ctx, userID, language); err != nil {
			h.reply(chatID, "Til saqlanbadı.")
			return
		}
		h.setLanguageCache(userID, language)
		if language == "ru" {
			h.reply(chatID, "Выбран русский язык.")
		} else {
			h.reply(chatID, "Qaraqalpaq tili tańlandı.")
		}
	case data == "menu_batch":
		h.startBatch(ctx, chatID, userID)
	case strings.HasPrefix(data, "batch_material_"):
		if !h.isCollector(userID) { h.reply(chatID, "Команда доступна сборщикам тестового пилота."); return }
		material := strings.TrimPrefix(data, "batch_material_")
		batch, err := h.workflowUC.StartBatch(ctx, userID, material)
		if err != nil { h.reply(chatID, "Не удалось открыть рейс: "+err.Error()); return }
		h.reply(chatID, fmt.Sprintf("🚚 Рейс %s открыт для фракции %s. Сканируйте QR контейнера камерой телефона. Завершить: /finishbatch.", batch.Code, materialLabel(material)))
	case strings.HasPrefix(data, "issue_bin_"):
		id, err := strconv.ParseUint(strings.TrimPrefix(data, "issue_bin_"), 10, 64)
		if err != nil { return }
		h.setSession(chatID, &session{step: stepIssueDescription, issueBinID: id})
		h.reply(chatID, "Кратко опишите пропуск: например, «по графику забирали сегодня, контейнер остался полным».")
	case strings.HasPrefix(data, "resolve_issue_"):
		if !h.isAdmin(userID) { h.reply(chatID, "Нет доступа."); return }
		id, err := strconv.ParseUint(strings.TrimPrefix(data, "resolve_issue_"), 10, 64)
		if err == nil { _ = h.workflowUC.ResolveIssue(ctx, id); h.reply(chatID, fmt.Sprintf("✅ Заявка #%d закрыта.", id)) }
	case strings.HasPrefix(data, "resolve_dump_"):
		if !h.isAdmin(userID) { h.reply(chatID, "Нет доступа."); return }
		id, err := strconv.ParseUint(strings.TrimPrefix(data, "resolve_dump_"), 10, 64)
		if err == nil { _ = h.workflowUC.ResolveDump(ctx, id); h.reply(chatID, fmt.Sprintf("✅ Обращение #%d закрыто.", id)) }
	case strings.HasPrefix(data, "reminders_"):
		enabled := strings.TrimPrefix(data, "reminders_") == "on"
		if err := h.workflowUC.SetReminders(ctx, userID, enabled); err != nil { h.reply(chatID, "Не удалось обновить настройку."); return }
		if enabled { h.reply(chatID, "🔔 Напоминания включены.") } else { h.reply(chatID, "🔕 Напоминания выключены.") }
	case strings.HasPrefix(data, "bin_full_") || strings.HasPrefix(data, "bin_empty_"):
		id, status, ok := parseBinCallback(data)
		if !ok { return }
		err := h.registerUC.UpdateBinStatus(ctx, userID, id, status)
		switch {
		case errors.Is(err, usecase.ErrNotOwner), errors.Is(err, usecase.ErrBinNotFound):
			h.reply(chatID, "Это не ваш контейнер.")
		case err != nil:
			log.Printf("update bin %d: %v", id, err); h.reply(chatID, "Не удалось обновить статус.")
		default:
			h.reply(chatID, fmt.Sprintf("✅ Статус контейнера #%d: %s", id, status))
		}
	}
}

func (h *Handler) startRegistration(chatID, telegramID int64) {
	h.setSession(chatID, &session{step: stepFullName, user: dto.RegisterUserInput{TelegramID: telegramID}})
	h.reply(chatID, "Давайте подключим дом к раздельному сбору. Как вас зовут?")
}

func (h *Handler) startDumpReport(chatID, telegramID int64) {
	h.setSession(chatID, &session{step: stepDumpPhoto, dump: dto.DumpReportInput{TelegramID: telegramID}})
	h.reply(chatID, "📸 Отправьте фотографию свалки.")
}

func (h *Handler) finishRegistration(ctx context.Context, chatID int64, sess *session) {
	bins, err := h.registerUC.Execute(ctx, sess.user)
	if errors.Is(err, usecase.ErrAlreadyRegistered) { h.finish(chatID, "Вы уже зарегистрированы ✅"); return }
	if err != nil { log.Printf("register: %v", err); h.reply(chatID, "Не удалось завершить регистрацию. Попробуйте снова."); return }
	for _, b := range bins {
		photo := tgbotapi.NewPhoto(chatID, tgbotapi.FilePath(b.QRPath))
		photo.Caption = materialLabel(b.Material) + " · QR-код контейнера"
		h.send(photo)
	}
	h.finish(chatID, "🏡 Дом подключён. Вы получили отдельный QR-код для каждой фракции.")
}

func (h *Handler) finish(chatID int64, text string) {
	h.reply(chatID, text)
	h.dropSession(chatID)
}

func (h *Handler) showUserBins(ctx context.Context, chatID, telegramID int64) {
	bins, err := h.registerUC.GetUserBins(ctx, telegramID)
	if errors.Is(err, usecase.ErrNotRegistered) { h.reply(chatID, "Сначала подключите дом через «Регистрация»."); return }
	if err != nil { log.Printf("get bins: %v", err); h.reply(chatID, "Не удалось загрузить контейнеры."); return }
	if len(bins) == 0 { h.reply(chatID, "Контейнеров пока нет."); return }
	var lines []string
	var rows [][]tgbotapi.InlineKeyboardButton
	for _, b := range bins {
		lines = append(lines, fmt.Sprintf("%s · %s · #%d", materialLabel(b.Material), b.Status, b.ID))
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🟠 Нужен вывоз", fmt.Sprintf("bin_full_%d", b.ID)),
			tgbotapi.NewInlineKeyboardButtonData("🟢 Пустой", fmt.Sprintf("bin_empty_%d", b.ID)),
		))
	}
	m := h.newMessage(chatID, "♻️ <b>Контейнерлер úyde</b>\n\n"+strings.Join(lines, "\n"))
	m.ParseMode = tgbotapi.ModeHTML
	m.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(rows...)
	h.send(m)
}

func (h *Handler) showSchedule(chatID int64) {
	if len(h.schedule) == 0 {
		h.reply(chatID, "🚚 График ещё не настроен. Оператор задаёт расписание через переменную PICKUP_SCHEDULE в формате mon:plastic,tue:paper,fri:glass.")
		return
	}
	var lines []string
	for day := time.Sunday; day <= time.Saturday; day++ {
		if materials := h.schedule[day]; len(materials) > 0 {
			var labels []string
			for _, material := range materials { labels = append(labels, materialLabel(material)) }
			lines = append(lines, fmt.Sprintf("• %s — %s", dayLabel(day), strings.Join(labels, ", ")))
		}
	}
	h.reply(chatID, "🚚 <b>График вывоза</b>\n\n"+strings.Join(lines, "\n")+"\n\nВключите напоминания командой /reminders.")
}

func (h *Handler) showSortingGuide(chatID int64) {
	h.reply(chatID, "🧭 <b>Что куда?</b>\n\n🧴 <b>Пластик</b> — пустая чистая упаковка; снимите остатки еды.\n📄 <b>Бумага</b> — сухая бумага и картон, без жирных и мокрых загрязнений.\n🍾 <b>Стекло</b> — бутылки и банки; керамику и лампы не кладите.\n\nТочный список принимает оператор и местная площадка переработки.")
}

func (h *Handler) showImpact(ctx context.Context, chatID, telegramID int64) {
	items, err := h.workflowUC.Impact(ctx, telegramID)
	if err != nil { h.reply(chatID, "Не удалось загрузить подтверждённые данные."); return }
	if len(items) == 0 { h.reply(chatID, "🌱 Пока нет принятого переработчиком материала. После закрытия рейса появится подтверждённый вес."); return }
	var lines []string
	var total float64
	for _, item := range items {
		lines = append(lines, fmt.Sprintf("%s — %.2f кг", materialLabel(item.Material), item.AcceptedKG))
		total += item.AcceptedKG
	}
	h.reply(chatID, fmt.Sprintf("🌱 <b>Подтверждённый экобаланс</b>\n\n%s\n\nВсего принято: %.2f кг", strings.Join(lines, "\n"), total))
}

func (h *Handler) startMissedPickup(ctx context.Context, chatID, telegramID int64) {
	bins, err := h.registerUC.GetUserBins(ctx, telegramID)
	if errors.Is(err, usecase.ErrNotRegistered) { h.reply(chatID, "Сначала подключите дом через «Регистрация»."); return }
	if err != nil || len(bins) == 0 { h.reply(chatID, "Не удалось загрузить ваши контейнеры."); return }
	rows := make([][]tgbotapi.InlineKeyboardButton, 0, len(bins))
	for _, b := range bins {
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(tgbotapi.NewInlineKeyboardButtonData(materialLabel(b.Material)+" · #"+strconv.FormatUint(b.ID, 10), "issue_bin_"+strconv.FormatUint(b.ID, 10))))
	}
	m := h.newMessage(chatID, "⚠️ Выберите контейнер, который не забрали:")
	m.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(rows...)
	h.send(m)
}

func (h *Handler) showUserReports(ctx context.Context, chatID, telegramID int64) {
	issues, err := h.workflowUC.UserIssues(ctx, telegramID)
	if err != nil { h.reply(chatID, "Не удалось загрузить заявки."); return }
	dumps, err := h.workflowUC.UserDumps(ctx, telegramID)
	if err != nil { h.reply(chatID, "Не удалось загрузить обращения."); return }
	if len(issues)+len(dumps) == 0 { h.reply(chatID, "У вас пока нет обращений."); return }
	var lines []string
	for _, item := range issues {
		lines = append(lines, fmt.Sprintf("⚠️ #%d · пропуск вывоза · %s · %s", item.ID, statusLabel(item.Status), html.EscapeString(item.Description)))
	}
	for _, item := range dumps {
		lines = append(lines, fmt.Sprintf("🗑 #%d · свалка · %s · %s", item.ID, statusLabel(item.Status), html.EscapeString(item.Address)))
	}
	h.reply(chatID, "📋 <b>Мои обращения</b>\n\n"+strings.Join(lines, "\n"))
}

func (h *Handler) startBatch(ctx context.Context, chatID int64, userID int64) {
	if !h.isCollector(userID) { h.reply(chatID, "Раздел доступен только сборщикам пилота."); return }
	batch, err := h.workflowUC.OpenBatch(ctx, userID)
	if err != nil { h.reply(chatID, "Не удалось проверить активный рейс."); return }
	if batch != nil {
		h.reply(chatID, fmt.Sprintf("🚚 Уже открыт рейс %s · %s · %.2f кг. Сканируйте QR или завершите /finishbatch.", batch.Code, materialLabel(batch.Material), batch.CollectedKG))
		return
	}
	rows := [][]tgbotapi.InlineKeyboardButton{
		tgbotapi.NewInlineKeyboardRow(tgbotapi.NewInlineKeyboardButtonData("🧴 Пластик", "batch_material_"+bin.MaterialPlastic)),
		tgbotapi.NewInlineKeyboardRow(tgbotapi.NewInlineKeyboardButtonData("📄 Бумага", "batch_material_"+bin.MaterialPaper)),
		tgbotapi.NewInlineKeyboardRow(tgbotapi.NewInlineKeyboardButtonData("🍾 Стекло", "batch_material_"+bin.MaterialGlass)),
	}
	m := h.newMessage(chatID, "Выберите фракцию для нового рейса:")
	m.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(rows...)
	h.send(m)
}

func (h *Handler) beginPickup(ctx context.Context, chatID, userID int64, qr string) {
	if !h.isCollector(userID) { h.reply(chatID, "QR-код доступен сборщикам пилота. Если вы житель, откройте свои контейнеры через /bins."); return }
	batch, err := h.workflowUC.OpenBatch(ctx, userID)
	if err != nil { h.reply(chatID, "Не удалось проверить рейс."); return }
	if batch == nil { h.reply(chatID, "Сначала откройте рейс командой /batch, выберите фракцию, затем повторно отсканируйте QR."); return }
	h.setSession(chatID, &session{step: stepPickupWeight, pickupQR: qr})
	h.reply(chatID, fmt.Sprintf("📦 Рейс %s · %s. Введите вес этого контейнера в кг.", batch.Code, materialLabel(batch.Material)))
}

func (h *Handler) finishBatch(ctx context.Context, chatID int64, userID int64) {
	if !h.isCollector(userID) { h.reply(chatID, "Команда доступна сборщикам пилота."); return }
	b, err := h.workflowUC.FinishBatch(ctx, userID)
	if err != nil { h.reply(chatID, "Не удалось завершить рейс: "+err.Error()); return }
	h.reply(chatID, fmt.Sprintf("📨 Рейс %s отправлен на приёмку · %s · %.2f кг. Получатель подтверждает вес командой /accept %s ЧИСЛО_КГ.", b.Code, materialLabel(b.Material), b.CollectedKG, b.Code))
}

func (h *Handler) acceptBatch(ctx context.Context, chatID int64, userID int64, args []string) {
	if !h.isRecycler(userID) { h.reply(chatID, "Команда доступна подтверждённому получателю или администратору."); return }
	if len(args) != 2 { h.reply(chatID, "Формат: /accept ECO-YYYYMMDD-00001 12.5"); return }
	weight, err := strconv.ParseFloat(strings.ReplaceAll(args[1], ",", "."), 64)
	if err != nil { h.reply(chatID, "Вес должен быть числом в килограммах."); return }
	b, err := h.workflowUC.AcceptBatch(ctx, args[0], userID, weight)
	if err != nil { h.reply(chatID, "Не удалось подтвердить рейс: "+err.Error()); return }
	h.reply(chatID, fmt.Sprintf("✅ Приёмка записана: %s · собрано %.2f кг · принято %.2f кг · статус %s.", b.Code, b.CollectedKG, b.AcceptedKG, statusLabel(b.Status)))
}

func (h *Handler) showOpenIssues(ctx context.Context, chatID, userID int64) {
	if !h.isAdmin(userID) { h.reply(chatID, "Нет доступа."); return }
	items, err := h.workflowUC.OpenIssues(ctx)
	if err != nil { h.reply(chatID, "Не удалось загрузить заявки."); return }
	if len(items) == 0 { h.reply(chatID, "Открытых заявок о пропуске вывоза нет."); return }
	for _, item := range items {
		m := h.newMessage(chatID, fmt.Sprintf("⚠️ #%d · контейнер #%d · %s\n%s\n%s", item.ID, item.BinID, materialLabel(item.Material), html.EscapeString(item.Address), html.EscapeString(item.Description)))
		m.ParseMode = tgbotapi.ModeHTML
		m.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(tgbotapi.NewInlineKeyboardRow(tgbotapi.NewInlineKeyboardButtonData("✅ Múrájattı jabıw", fmt.Sprintf("resolve_issue_%d", item.ID))))
		h.send(m)
	}
}

func (h *Handler) showOpenDumps(ctx context.Context, chatID, userID int64) {
	if !h.isAdmin(userID) { h.reply(chatID, "Нет доступа."); return }
	items, err := h.workflowUC.OpenDumps(ctx)
	if err != nil { h.reply(chatID, "Не удалось загрузить обращения."); return }
	if len(items) == 0 { h.reply(chatID, "Открытых сообщений о свалках нет."); return }
	for _, item := range items {
		m := h.newMessage(chatID, fmt.Sprintf("🗑 #%d · %s\n%s", item.ID, html.EscapeString(item.Address), html.EscapeString(item.Description)))
		m.ParseMode = tgbotapi.ModeHTML
		m.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(tgbotapi.NewInlineKeyboardRow(tgbotapi.NewInlineKeyboardButtonData("✅ Tazalandı dep belgilew", fmt.Sprintf("resolve_dump_%d", item.ID))))
		h.send(m)
	}
}

func (h *Handler) resolveIssue(ctx context.Context, chatID, userID int64, args []string) {
	if !h.isAdmin(userID) { h.reply(chatID, "Нет доступа."); return }
	if len(args) != 1 { h.reply(chatID, "Формат: /resolve_issue 123"); return }
	id, err := strconv.ParseUint(args[0], 10, 64)
	if err != nil { h.reply(chatID, "Номер заявки должен быть числом."); return }
	if err := h.workflowUC.ResolveIssue(ctx, id); err != nil { h.reply(chatID, "Не удалось закрыть заявку."); return }
	h.reply(chatID, fmt.Sprintf("✅ Заявка #%d закрыта.", id))
}

func (h *Handler) resolveDump(ctx context.Context, chatID, userID int64, args []string) {
	if !h.isAdmin(userID) { h.reply(chatID, "Нет доступа."); return }
	if len(args) != 1 { h.reply(chatID, "Формат: /resolve_dump 123"); return }
	id, err := strconv.ParseUint(args[0], 10, 64)
	if err != nil { h.reply(chatID, "Номер обращения должен быть числом."); return }
	if err := h.workflowUC.ResolveDump(ctx, id); err != nil { h.reply(chatID, "Не удалось закрыть обращение."); return }
	h.reply(chatID, fmt.Sprintf("✅ Обращение #%d закрыто.", id))
}

func (h *Handler) handleReminders(ctx context.Context, chatID, userID int64, args []string) {
	if len(args) > 0 {
		enabled := strings.EqualFold(args[0], "on")
		if !enabled && !strings.EqualFold(args[0], "off") { h.reply(chatID, "Используйте /reminders on или /reminders off."); return }
		if err := h.workflowUC.SetReminders(ctx, userID, enabled); err != nil { h.reply(chatID, "Не удалось обновить настройку."); return }
		if enabled { h.reply(chatID, "🔔 Напоминания включены.") } else { h.reply(chatID, "🔕 Напоминания выключены.") }
		return
	}
	m := h.newMessage(chatID, "🔔 Напоминания приходят вечером накануне вывоза.")
	m.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(tgbotapi.NewInlineKeyboardButtonData("Включить", "reminders_on"), tgbotapi.NewInlineKeyboardButtonData("Выключить", "reminders_off")),
	)
	h.send(m)
}

func (h *Handler) showAdminHelp(chatID, userID int64) {
	if !h.isAdmin(userID) && !h.isCollector(userID) && !h.isRecycler(userID) { h.reply(chatID, "У вас нет доступа к ролям пилота."); return }
	h.reply(chatID, "🛠 <b>Команды пилота</b>\n\nСборщик: /batch → QR → вес → /finishbatch\nПолучатель: /accept ECO-... ПРИНЯТЫЙ_КГ\nАдминистратор: /issues, /dumps, /resolve_issue ID, /resolve_dump ID\n\nРоли задаются через ADMIN_TELEGRAM_IDS, COLLECTOR_TELEGRAM_IDS и RECYCLER_TELEGRAM_IDS.")
}

func (h *Handler) showLanguage(chatID int64) {
	m := tgbotapi.NewMessage(chatID, "Til / Язык · tildi tańlań")
	m.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData("🇰🇰 Qaraqalpaqsha", "language_kaa"),
		tgbotapi.NewInlineKeyboardButtonData("🇷🇺 Русский", "language_ru"),
	))
	h.send(m)
}

func (h *Handler) reminderLoop(ctx context.Context) {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	lastSent := ""
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if len(h.schedule) == 0 { continue }
			now := time.Now().In(h.location)
			if now.Hour() != h.reminderHour || now.Minute() != 0 { continue }
			dateKey := now.Format("2006-01-02")
			if lastSent == dateKey { continue }
			day := now.AddDate(0, 0, 1).Weekday()
			materials := h.schedule[day]
			if len(materials) == 0 { lastSent = dateKey; continue }
			recipients, err := h.workflowUC.ReminderRecipients(ctx)
			if err != nil { log.Printf("reminder recipients: %v", err); continue }
			labels := make([]string, 0, len(materials))
			for _, material := range materials { labels = append(labels, materialLabel(material)) }
			for _, id := range recipients {
				h.reply(id, "🔔 Завтра вывоз: "+strings.Join(labels, ", ")+". Подготовьте соответствующие контейнеры.")
			}
			lastSent = dateKey
		}
	}
}

func (h *Handler) isAdmin(id int64) bool { return h.admins[id] }
func (h *Handler) isCollector(id int64) bool { return h.admins[id] || h.collectors[id] }
func (h *Handler) isRecycler(id int64) bool { return h.admins[id] || h.recyclers[id] }

func (h *Handler) setSession(chatID int64, s *session) { h.mu.Lock(); h.sessions[chatID] = s; h.mu.Unlock() }
func (h *Handler) dropSession(chatID int64) { h.mu.Lock(); delete(h.sessions, chatID); h.mu.Unlock() }

func mainMenu(language string) tgbotapi.ReplyKeyboardMarkup {
	labels := []string{btnMyBins, btnSchedule, btnMissed, btnDump, btnImpact, btnSort, btnIssues, btnReminders, btnRegister, btnLanguage, btnHelp}
	if language == "ru" {
		labels = []string{"♻️ Мои контейнеры", "🚚 График вывоза", "⚠️ Не забрали", "🗑 Сообщить о свалке", "🌱 Экобаланс", "🧭 Что куда?", "📋 Мои обращения", "🔔 Напоминания", "🧍 Регистрация", "🗣 Язык / Til", "ℹ️ Помощь"}
	}
	m := tgbotapi.NewReplyKeyboard(
		tgbotapi.NewKeyboardButtonRow(tgbotapi.NewKeyboardButton(labels[0]), tgbotapi.NewKeyboardButton(labels[1])),
		tgbotapi.NewKeyboardButtonRow(tgbotapi.NewKeyboardButton(labels[2]), tgbotapi.NewKeyboardButton(labels[3])),
		tgbotapi.NewKeyboardButtonRow(tgbotapi.NewKeyboardButton(labels[4]), tgbotapi.NewKeyboardButton(labels[5])),
		tgbotapi.NewKeyboardButtonRow(tgbotapi.NewKeyboardButton(labels[6]), tgbotapi.NewKeyboardButton(labels[7])),
		tgbotapi.NewKeyboardButtonRow(tgbotapi.NewKeyboardButton(labels[8]), tgbotapi.NewKeyboardButton(labels[9])),
		tgbotapi.NewKeyboardButtonRow(tgbotapi.NewKeyboardButton(labels[10])),
	)
	m.ResizeKeyboard = true
	return m
}

func parseBinCallback(data string) (uint64, string, bool) {
	var id uint64
	if _, err := fmt.Sscanf(data, "bin_full_%d", &id); err == nil { return id, bin.StatusFull, true }
	if _, err := fmt.Sscanf(data, "bin_empty_%d", &id); err == nil { return id, bin.StatusEmpty, true }
	return 0, "", false
}

func parseIDs(raw string) map[int64]bool {
	out := make(map[int64]bool)
	for _, item := range strings.Split(raw, ",") {
		if id, err := strconv.ParseInt(strings.TrimSpace(item), 10, 64); err == nil && id != 0 { out[id] = true }
	}
	return out
}

func parseSchedule(raw string) map[time.Weekday][]string {
	out := make(map[time.Weekday][]string)
	dayMap := map[string]time.Weekday{
		"sun": time.Sunday, "sunday": time.Sunday, "mon": time.Monday, "monday": time.Monday,
		"tue": time.Tuesday, "tuesday": time.Tuesday, "wed": time.Wednesday, "wednesday": time.Wednesday,
		"thu": time.Thursday, "thursday": time.Thursday, "fri": time.Friday, "friday": time.Friday,
		"sat": time.Saturday, "saturday": time.Saturday,
	}
	for _, entry := range strings.Split(raw, ",") {
		pair := strings.SplitN(strings.TrimSpace(entry), ":", 2)
		if len(pair) != 2 { continue }
		day, ok := dayMap[strings.ToLower(strings.TrimSpace(pair[0]))]
		material := strings.ToLower(strings.TrimSpace(pair[1]))
		if !ok || (material != bin.MaterialPlastic && material != bin.MaterialPaper && material != bin.MaterialGlass) { continue }
		out[day] = append(out[day], material)
	}
	return out
}

func materialLabel(material string) string {
	switch material {
	case bin.MaterialPlastic: return "🧴 Пластик"
	case bin.MaterialPaper: return "📄 Бумага"
	case bin.MaterialGlass: return "🍾 Стекло"
	default: return "♻️ Смешанная фракция"
	}
}

func dayLabel(day time.Weekday) string {
	return []string{"Воскресенье", "Понедельник", "Вторник", "Среда", "Четверг", "Пятница", "Суббота"}[int(day)]
}

func statusLabel(status string) string {
	switch status {
	case "new": return "новое"
	case "assigned": return "передано оператору"
	case "resolved": return "решено"
	case "open": return "открыт"
	case "submitted": return "ожидает приёмки"
	case "accepted": return "принято"
	case "partial": return "принято частично"
	case "rejected": return "не принято"
	default: return status
	}
}
