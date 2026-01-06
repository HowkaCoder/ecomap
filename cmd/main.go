package main

import (
	"log"
	"os"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"ecomap/internal/adapter/postgres"
	"ecomap/internal/adapter/qr"
	"ecomap/internal/handler/telegram"
	"ecomap/internal/usecase"
	"ecomap/internal/adapter/photo"
)
func main() {
	// 1️⃣ Читаем env
	botToken := os.Getenv("BOT_TOKEN")
	dbURL := os.Getenv("DATABASE_URL")

	if botToken == "" || dbURL == "" {
		log.Fatal("BOT_TOKEN or DATABASE_URL not set")
	}

	// 2️⃣ Telegram bot
	bot, err := tgbotapi.NewBotAPI(botToken)
	if err != nil {
		log.Fatal(err)
	}
	bot.Debug = false

	log.Printf("Authorized as @%s", bot.Self.UserName)

	// 3️⃣ PostgreSQL (pgx pool)
	// ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	// defer cancel()

	db, err := postgres.NewPool(dbURL)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	// 4️⃣ Repositories
	userRepo := postgres.NewUserRepo(db)
	binRepo := postgres.NewBinRepo(db)
	dumpRepo := postgres.NewDumpRepo(db)
	photoRepo := photo.New(bot , "photos")
	// 5️⃣ Services
	qrService := qr.New("storage")

	// 6️⃣ Usecases
	registerUserUC := usecase.NewRegisterUserUseCase(
		userRepo,
		binRepo,
		qrService,
		photoRepo,
	)

	DumpRepoUsecaseUC := usecase.NewDumpReportUsecase(userRepo , dumpRepo , photoRepo)
	// 7️⃣ Telegram handler
	handler := telegram.New(
		bot,
		registerUserUC,
		DumpRepoUsecaseUC,
	)

	log.Println("🚀 Bot started")
	handler.Start()
}
