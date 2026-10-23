package telegram

import (
	"context"
	"errors"
	"fmt"
	"log"
	"sync"

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
	stepBinLocation

	stepDumpPhoto
	stepDumpLocation
	stepDumpAddress
	stepDumpLandmark
	stepDumpDesc
	stepDumpStatus
)

const (
	btnRegister = "🧍 Регистрация"
	btnDump     = "🗑 Сообщить о свалке"
	btnHelp     = "ℹ️ Помощь"
	btnMyBins   = "♻️ Моя урна"
)

type session struct {
	step step
	user dto.RegisterUserInput
	dump dto.DumpReportInput
}

type Handler struct {
	bot        *tgbotapi.BotAPI
	registerUC *usecase.RegisterUserUseCase
	dumpUC     *usecase.DumpReportUsecase

	mu       sync.Mutex
	sessions map[int64]*session
}

func New(bot *tgbotapi.BotAPI, registerUC *usecase.RegisterUserUseCase, dumpUC *usecase.DumpReportUsecase) *Handler {
	return &Handler{
		bot:        bot,
		registerUC: registerUC,
		dumpUC:     dumpUC,
		sessions:   make(map[int64]*session),
	}
}

func (h *Handler) Start(ctx context.Context) {
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
	switch {
	case update.CallbackQuery != nil:
		h.handleCallback(ctx, update.CallbackQuery)
	case update.Message == nil:
		return
	case update.Message.IsCommand():
		h.handleCommand(update.Message)
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
	h.send(tgbotapi.NewMessage(chatID, text))
}

func (h *Handler) handleCommand(msg *tgbotapi.Message) {
	if msg.Command() != "start" {
		return
	}
	h.dropSession(msg.Chat.ID)

	m := tgbotapi.NewMessage(msg.Chat.ID, "🌱 *EcoMap*\n\nВыберите действие:")
	m.ParseMode = tgbotapi.ModeMarkdown
	m.ReplyMarkup = mainMenu()
	h.send(m)
}

func (h *Handler) handleMessage(ctx context.Context, msg *tgbotapi.Message) {
	chatID := msg.Chat.ID

	switch msg.Text {
	case btnMyBins:
		h.dropSession(chatID)
		h.showUserBins(ctx, chatID, msg.From.ID)
		return
	case btnRegister:
		h.startRegistration(chatID, msg.From.ID)
		return
	case btnDump:
		h.startDumpReport(chatID, msg.From.ID)
		return
	case btnHelp:
		h.dropSession(chatID)
		h.reply(chatID, "Вы можете зарегистрироваться или сообщить о свалке.\nПросто выберите действие.")
		return
	}

	h.mu.Lock()
	sess, ok := h.sessions[chatID]
	h.mu.Unlock()

	if !ok {
		m := tgbotapi.NewMessage(chatID, "Нажмите /start")
		m.ReplyMarkup = mainMenu()
		h.send(m)
		return
	}

	switch sess.step {
	case stepDumpPhoto:
		if len(msg.Photo) == 0 {
			h.reply(chatID, "📸 Пожалуйста, отправьте фотографию свалки")
			return
		}
		sess.dump.PhotoID = msg.Photo[len(msg.Photo)-1].FileID
		sess.step = stepDumpLocation
		h.reply(chatID, "📍 Отправьте геолокацию свалки")

	case stepDumpLocation:
		if msg.Location == nil {
			h.reply(chatID, "📍 Нужна геолокация")
			return
		}
		sess.dump.Lat = msg.Location.Latitude
		sess.dump.Lon = msg.Location.Longitude
		sess.step = stepDumpAddress
		h.reply(chatID, "🏠 Укажите адрес (если знаете)")

	case stepDumpAddress:
		sess.dump.Address = msg.Text
		sess.step = stepDumpLandmark
		h.reply(chatID, "🧭 Добавьте ориентир (школа, магазин и т.д.)")

	case stepDumpLandmark:
		sess.dump.Landmark = msg.Text
		sess.step = stepDumpDesc
		h.reply(chatID, "📝 Кратко опишите проблему")

	case stepDumpDesc:
		sess.dump.Description = msg.Text
		sess.step = stepDumpStatus
		h.reply(chatID, "🚨 Состояние свалки (обычная / опасная / срочно)")

	case stepDumpStatus:
		sess.dump.Status = msg.Text
		err := h.dumpUC.Execute(ctx, sess.dump)
		if errors.Is(err, usecase.ErrNotRegistered) {
			h.finish(chatID, "❗ Сначала нужно зарегистрироваться")
			return
		}
		if err != nil {
			log.Printf("dump report: %v", err)
			h.reply(chatID, "❌ Ошибка при сохранении жалобы, попробуйте ещё раз")
			return
		}
		h.finish(chatID, "✅ Жалоба принята. Спасибо!")

	case stepFullName:
		if msg.Text == "" {
			h.reply(chatID, "Введите ваше ФИО текстом")
			return
		}
		sess.user.FullName = msg.Text
		sess.step = stepBinPhoto
		h.reply(chatID, "📸 Отправьте фото урны")

	case stepBinPhoto:
		if len(msg.Photo) == 0 {
			h.reply(chatID, "📸 Нужна фотография")
			return
		}
		sess.user.BinPhoto = msg.Photo[len(msg.Photo)-1].FileID
		sess.step = stepBinLocation
		h.reply(chatID, "📍 Отправьте геолокацию")

	case stepBinLocation:
		if msg.Location == nil {
			h.reply(chatID, "📍 Нужна геолокация")
			return
		}
		sess.user.Lat = msg.Location.Latitude
		sess.user.Lon = msg.Location.Longitude
		h.finishRegistration(ctx, chatID, sess)
	}
}

func (h *Handler) startRegistration(chatID, telegramID int64) {
	h.setSession(chatID, &session{
		step: stepFullName,
		user: dto.RegisterUserInput{TelegramID: telegramID},
	})
	h.reply(chatID, "Введите ваше ФИО:")
}

func (h *Handler) startDumpReport(chatID, telegramID int64) {
	h.setSession(chatID, &session{
		step: stepDumpPhoto,
		dump: dto.DumpReportInput{TelegramID: telegramID},
	})
	h.reply(chatID, "📸 Отправьте фотографию свалки")
}

func (h *Handler) finishRegistration(ctx context.Context, chatID int64, sess *session) {
	qrPath, err := h.registerUC.Execute(ctx, sess.user)
	if errors.Is(err, usecase.ErrAlreadyRegistered) {
		h.finish(chatID, "Вы уже зарегистрированы ✅")
		return
	}
	if err != nil {
		log.Printf("register: %v", err)
		h.reply(chatID, "❌ Ошибка регистрации, попробуйте ещё раз")
		return
	}

	h.send(tgbotapi.NewPhoto(chatID, tgbotapi.FilePath(qrPath)))
	h.finish(chatID, "✅ Регистрация завершена\nВаш QR-код готов")
}

func (h *Handler) finish(chatID int64, text string) {
	h.reply(chatID, text)

	m := tgbotapi.NewMessage(chatID, "Что делаем дальше?")
	m.ReplyMarkup = mainMenu()
	h.send(m)

	h.dropSession(chatID)
}

func (h *Handler) showUserBins(ctx context.Context, chatID, telegramID int64) {
	bins, err := h.registerUC.GetUserBins(ctx, telegramID)
	if errors.Is(err, usecase.ErrNotRegistered) {
		h.reply(chatID, "❗ Сначала нужно зарегистрироваться")
		return
	}
	if err != nil {
		log.Printf("get bins: %v", err)
		h.reply(chatID, "❌ Не удалось загрузить урны")
		return
	}
	if len(bins) == 0 {
		h.reply(chatID, "У вас пока нет урн")
		return
	}

	text := "Ваши урны:\n"
	var rows [][]tgbotapi.InlineKeyboardButton
	for _, b := range bins {
		text += fmt.Sprintf("📦 ID: %d — Статус: %s\n", b.ID, b.Status)
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("✅ Заполнена", fmt.Sprintf("bin_full_%d", b.ID)),
			tgbotapi.NewInlineKeyboardButtonData("♻️ Пустая", fmt.Sprintf("bin_empty_%d", b.ID)),
		))
	}

	m := tgbotapi.NewMessage(chatID, text)
	m.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(rows...)
	h.send(m)
}

func (h *Handler) handleCallback(ctx context.Context, cb *tgbotapi.CallbackQuery) {
	if _, err := h.bot.Request(tgbotapi.NewCallback(cb.ID, "")); err != nil {
		log.Printf("answer callback: %v", err)
	}
	if cb.Message == nil {
		return
	}
	chatID := cb.Message.Chat.ID

	binID, status, ok := parseBinCallback(cb.Data)
	if !ok {
		return
	}

	err := h.registerUC.UpdateBinStatus(ctx, cb.From.ID, binID, status)
	switch {
	case errors.Is(err, usecase.ErrNotOwner), errors.Is(err, usecase.ErrBinNotFound):
		h.reply(chatID, "❌ Это не ваша урна")
	case err != nil:
		log.Printf("update bin %d: %v", binID, err)
		h.reply(chatID, "❌ Не удалось обновить статус")
	default:
		h.reply(chatID, fmt.Sprintf("✅ Статус урны #%d обновлён на: %s", binID, status))
	}
}

func parseBinCallback(data string) (uint64, string, bool) {
	var id uint64
	if _, err := fmt.Sscanf(data, "bin_full_%d", &id); err == nil {
		return id, bin.StatusFull, true
	}
	if _, err := fmt.Sscanf(data, "bin_empty_%d", &id); err == nil {
		return id, bin.StatusEmpty, true
	}
	return 0, "", false
}

func (h *Handler) setSession(chatID int64, s *session) {
	h.mu.Lock()
	h.sessions[chatID] = s
	h.mu.Unlock()
}

func (h *Handler) dropSession(chatID int64) {
	h.mu.Lock()
	delete(h.sessions, chatID)
	h.mu.Unlock()
}

func mainMenu() tgbotapi.ReplyKeyboardMarkup {
	return tgbotapi.NewReplyKeyboard(
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton(btnRegister),
			tgbotapi.NewKeyboardButton(btnDump),
		),
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton(btnHelp),
			tgbotapi.NewKeyboardButton(btnMyBins),
		),
	)
}
