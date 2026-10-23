package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"ecomap/internal/adapter/photo"
	"ecomap/internal/adapter/postgres"
	"ecomap/internal/adapter/qr"
	"ecomap/internal/handler/telegram"
	"ecomap/internal/usecase"
)

func main() {
	botToken := os.Getenv("BOT_TOKEN")
	dbURL := os.Getenv("DATABASE_URL")
	storageDir := os.Getenv("STORAGE_DIR")
	if storageDir == "" {
		storageDir = "storage"
	}
	if botToken == "" || dbURL == "" {
		log.Fatal("BOT_TOKEN and DATABASE_URL must be set")
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	bot, err := tgbotapi.NewBotAPI(botToken)
	if err != nil {
		log.Fatalf("telegram: %v", err)
	}
	log.Printf("authorized as @%s", bot.Self.UserName)

	db, err := postgres.NewPool(ctx, dbURL)
	if err != nil {
		log.Fatalf("postgres: %v", err)
	}
	defer db.Close()

	users := postgres.NewUserRepo(db)
	bins := postgres.NewBinRepo(db)
	dumps := postgres.NewDumpRepo(db)
	photos := photo.New(bot, storageDir)

	registerUC := usecase.NewRegisterUserUseCase(users, bins, qr.New(storageDir), photos)
	dumpUC := usecase.NewDumpReportUsecase(users, dumps, photos)

	log.Println("bot started")
	telegram.New(bot, registerUC, dumpUC).Start(ctx)
	log.Println("bot stopped")
}
