package telegram

import (
	"context"
	"log"
	"sync"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"ecomap/internal/dto"
	"ecomap/internal/usecase"
	"fmt"
)



type step int

const (
	stepNone step = iota

	// registration
	stepFullName
	stepBinPhoto
	stepBinLocation

	// dump report
	stepDumpPhoto
	stepDumpLocation
	stepDumpAddress
	stepDumpLandmark
	stepDumpDesc
	stepDumpStatus
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

func New(
	bot *tgbotapi.BotAPI,
	registerUC *usecase.RegisterUserUseCase,
	dumpUC *usecase.DumpReportUsecase,
) *Handler {
	return &Handler{
		bot:        bot,
		registerUC: registerUC,
		dumpUC:     dumpUC,
		sessions:   make(map[int64]*session),
	}
}



func (h *Handler) Start() {
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates := h.bot.GetUpdatesChan(u)

	for update := range updates {
		if update.Message == nil {
			continue
		}

		if update.Message.IsCommand() {
			h.handleCommand(update)
			continue
		}

		for update := range updates {
			if update.Message != nil {
				h.handleMessage(update)
			} else if update.CallbackQuery != nil {
				h.handleCallback(update)
			}
		}
		
	}
}



func (h *Handler) handleCommand(update tgbotapi.Update) {
	if update.Message.Command() != "start" {
		return
	}

	msg := tgbotapi.NewMessage(
		update.Message.Chat.ID,
		"🌱 *EcoMap*\n\nВыберите действие:",
	)
	msg.ParseMode = "Markdown"
	msg.ReplyMarkup = mainMenu()

	h.bot.Send(msg)
}


func (h *Handler) handleMessage(update tgbotapi.Update) {
	chatID := update.Message.Chat.ID

	// кнопки главного меню
	switch update.Message.Text {
	case "♻️ Моя урна":
		h.showUserBins(update)
	
	case "🧍 Регистрация":
		h.startRegistration(update)
		return

	case "🗑 Сообщить о свалке":
		h.startDumpReport(update)
		return

	case "ℹ️ Помощь":
		h.bot.Send(tgbotapi.NewMessage(
			chatID,
			"Вы можете зарегистрироваться или сообщить о свалке.\nПросто выберите действие.",
		))
		return
	}

	// продолжаем сценарий
	h.mu.Lock()
	sess, ok := h.sessions[chatID]
	h.mu.Unlock()

	if !ok {
		msg := tgbotapi.NewMessage(chatID, "Нажмите /start")
		msg.ReplyMarkup = mainMenu()
		h.bot.Send(msg)
		return
	}

	switch sess.step {

	/* ===== DUMP REPORT ===== */

	case stepDumpPhoto:
		if len(update.Message.Photo) == 0 {
			h.bot.Send(tgbotapi.NewMessage(chatID, "📸 Пожалуйста, отправьте фотографию свалки"))
			return
		}

		photo := update.Message.Photo[len(update.Message.Photo)-1]
		sess.dump.PhotoURL = photo.FileID
		sess.step = stepDumpLocation

		h.bot.Send(tgbotapi.NewMessage(chatID, "📍 Отправьте геолокацию свалки"))

	case stepDumpLocation:
		if update.Message.Location == nil {
			h.bot.Send(tgbotapi.NewMessage(chatID, "📍 Нужна геолокация"))
			return
		}

		sess.dump.Lat = update.Message.Location.Latitude
		sess.dump.Lon = update.Message.Location.Longitude
		sess.step = stepDumpAddress

		h.bot.Send(tgbotapi.NewMessage(chatID, "🏠 Укажите адрес (если знаете)"))

	case stepDumpAddress:
		sess.dump.Address = update.Message.Text
		sess.step = stepDumpLandmark

		h.bot.Send(tgbotapi.NewMessage(chatID, "🧭 Добавьте ориентир (школа, магазин и т.д.)"))

	case stepDumpLandmark:
		sess.dump.Landmark = update.Message.Text
		sess.step = stepDumpDesc

		h.bot.Send(tgbotapi.NewMessage(chatID, "📝 Кратко опишите проблему"))

	case stepDumpDesc:
		sess.dump.Description = update.Message.Text
		sess.step = stepDumpStatus

		h.bot.Send(tgbotapi.NewMessage(chatID, "🚨 Состояние свалки (обычная / опасная / срочно)"))

	case stepDumpStatus:
		sess.dump.Status = update.Message.Text
		sess.dump.TelegramID = update.Message.From.ID

		if err := h.dumpUC.Execute(context.Background(), sess.dump); err != nil {
			log.Println(err)
			h.bot.Send(tgbotapi.NewMessage(chatID, "❌ Ошибка при сохранении жалобы"))
			return
		}

		h.finish(chatID, "✅ Жалоба принята. Спасибо!")



	case stepFullName:
		sess.user.FullName = update.Message.Text
		sess.step = stepBinPhoto

		h.bot.Send(tgbotapi.NewMessage(chatID, "📸 Отправьте фото урны"))

	case stepBinPhoto:
		if len(update.Message.Photo) == 0 {
			h.bot.Send(tgbotapi.NewMessage(chatID, "📸 Нужна фотография"))
			return
		}

		photo := update.Message.Photo[len(update.Message.Photo)-1]
		sess.user.BinPhoto = photo.FileID
		sess.step = stepBinLocation

		h.bot.Send(tgbotapi.NewMessage(chatID, "📍 Отправьте геолокацию"))

	case stepBinLocation:
		if update.Message.Location == nil {
			h.bot.Send(tgbotapi.NewMessage(chatID, "📍 Нужна геолокация"))
			return
		}

		sess.user.Lat = update.Message.Location.Latitude
		sess.user.Lon = update.Message.Location.Longitude

		h.finishRegistration(chatID, sess)
	}
}


func (h *Handler) startRegistration(update tgbotapi.Update) {
	chatID := update.Message.Chat.ID

	h.mu.Lock()
	h.sessions[chatID] = &session{
		step: stepFullName,
		user: dto.RegisterUserInput{
			TelegramID: update.Message.From.ID,
		},
	}
	h.mu.Unlock()

	h.bot.Send(tgbotapi.NewMessage(chatID, "Введите ваше ФИО:"))
}

func (h *Handler) startDumpReport(update tgbotapi.Update) {
	chatID := update.Message.Chat.ID

	h.mu.Lock()
	h.sessions[chatID] = &session{
		step: stepDumpPhoto,
		dump: dto.DumpReportInput{
			TelegramID: update.Message.From.ID,
		},
	}
	h.mu.Unlock()

	h.bot.Send(tgbotapi.NewMessage(chatID, "📸 Отправьте фотографию свалки"))
}

func (h *Handler) finishRegistration(chatID int64, sess *session) {
	ctx := context.Background()

	exists, err := h.registerUC.UserExists(ctx, sess.user.TelegramID)
	if err != nil {
		log.Println(err)
		h.bot.Send(tgbotapi.NewMessage(chatID, "❌ Ошибка проверки пользователя"))
		return
	}

	if exists {
		h.finish(chatID, "Вы уже зарегистрированы ✅")
		return
	}       

	qrPath, err := h.registerUC.Execute(ctx, sess.user)
	if err != nil {
		log.Println(err)
		h.bot.Send(tgbotapi.NewMessage(chatID, "❌ Ошибка регистрации"))
		return
	}

	h.bot.Send(tgbotapi.NewPhoto(chatID, tgbotapi.FilePath(qrPath)))
	h.finish(chatID, "✅ Регистрация завершена\nВаш QR-код готов")
}

func (h *Handler) finish(chatID int64, text string) {
	h.bot.Send(tgbotapi.NewMessage(chatID, text))

	msg := tgbotapi.NewMessage(chatID, "Что делаем дальше?")
	msg.ReplyMarkup = mainMenu()
	h.bot.Send(msg)

	h.mu.Lock()
	delete(h.sessions, chatID)
	h.mu.Unlock()
}


func (h *Handler) showUserBins(update tgbotapi.Update) {
	chatID := update.Message.Chat.ID
	go func(chatID int64, userID int64) {
		ctx := context.Background()
		bins, err := h.registerUC.GetUserBins(ctx, userID)
		if err != nil {
			h.bot.Send(tgbotapi.NewMessage(chatID, "❗ Сначала нужно зарегистрироваться"))
			return
		}

		if len(bins) == 0 {
			h.bot.Send(tgbotapi.NewMessage(chatID, "У вас пока нет урн"))
			return
		}

		text := "Ваши урны:\n"
		var rows [][]tgbotapi.InlineKeyboardButton

		for _, b := range bins {
			text += fmt.Sprintf("📦 ID: %d — Статус: %s\n", b.ID, b.Status)
			row := tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("✅ Заполнена", fmt.Sprintf("bin_full_%d", b.ID)),
				tgbotapi.NewInlineKeyboardButtonData("♻️ Пустая", fmt.Sprintf("bin_empty_%d", b.ID)),
			)
			rows = append(rows, row)
		}

		msg := tgbotapi.NewMessage(chatID, text)
		msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(rows...)
		h.bot.Send(msg)
	}(chatID, update.Message.From.ID)
}



func (h *Handler) handleCallback(update tgbotapi.Update) {
	callback := update.CallbackQuery
	if callback == nil {
		return
	}

	data := callback.Data 
	var binID int64
	var newStatus string

	if _, err := fmt.Sscanf(data, "bin_full_%d", &binID); err == nil {
		newStatus = "Нужна уборка"
	} else if _, err := fmt.Sscanf(data, "bin_empty_%d", &binID); err == nil {
		newStatus = "Пустая"
	} else {
		return
	}

	ctx := context.Background()
	log.Println("here is updating of bin status")
	if err := h.registerUC.UpdateBinStatus(ctx ,  binID, newStatus); err != nil {
		h.bot.Send(tgbotapi.NewMessage(update.CallbackQuery.Message.Chat.ID, "❌ Не удалось обновить статус"))
		return
	}
	msg := tgbotapi.NewMessage(update.CallbackQuery.Message.Chat.ID,
		fmt.Sprintf("✅ Статус урны #%d обновлён на: %s", binID, newStatus))
	h.bot.Send(msg)

	h.bot.Request(tgbotapi.NewCallback(callback.ID, ""))
}


func mainMenu() tgbotapi.ReplyKeyboardMarkup {
	return tgbotapi.NewReplyKeyboard(
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton("🧍 Регистрация"),
			tgbotapi.NewKeyboardButton("🗑 Сообщить о свалке"),
			
		),
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton("ℹ️ Помощь"),
			tgbotapi.NewKeyboardButton("♻️ Моя урна"),

		),
	)
}
