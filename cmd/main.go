package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"ecomap/internal/adapter/photo"
	"ecomap/internal/adapter/postgres"
	"ecomap/internal/adapter/qr"
	"ecomap/internal/handler/telegram"
	"ecomap/internal/handler/webapp"
	"ecomap/internal/usecase"
)

func main() {
	botToken := os.Getenv("BOT_TOKEN")
	dbURL := os.Getenv("DATABASE_URL")
	storageDir := os.Getenv("STORAGE_DIR")
	if storageDir == "" {
		storageDir = "storage"
	}
	if dbURL == "" { log.Fatal("DATABASE_URL must be set") }

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	db, err := postgres.NewPool(ctx, dbURL)
	if err != nil {
		log.Fatalf("postgres: %v", err)
	}
	defer db.Close()
	if err := postgres.ApplyMigrations(ctx, db); err != nil { log.Fatalf("database migrations: %v", err) }

	port := os.Getenv("PORT")
	if port == "" { port = "8080" }
	adminIDs := os.Getenv("ADMIN_TELEGRAM_IDS")
	server := &http.Server{Addr: ":" + port, Handler: webapp.New(db, botToken, adminIDs, os.Getenv("PICKUP_SCHEDULE"), envOr("TIMEZONE", "Asia/Tashkent")), ReadHeaderTimeout: 5 * time.Second}
	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("health server: %v", err)
		}
	}()

	if botToken == "" {
		log.Println("BOT_TOKEN is not set; health endpoint is live and the Telegram bot is waiting for configuration")
		<-ctx.Done()
		shutdownHealthServer(server)
		return
	}

	bot, err := tgbotapi.NewBotAPI(botToken)
	if err != nil {
		log.Fatalf("telegram: %v", err)
	}
	log.Printf("authorized as @%s", bot.Self.UserName)

	publicDomain := os.Getenv("RAILWAY_PUBLIC_DOMAIN")
	miniAppURL := envOr("MINI_APP_URL", "https://"+publicDomain+"/miniapp/")
	users := postgres.NewUserRepo(db)
	bins := postgres.NewBinRepo(db)
	dumps := postgres.NewDumpRepo(db)
	workflows := postgres.NewWorkflowRepo(db)
	photos := photo.New(bot, storageDir)

	registerUC := usecase.NewRegisterUserUseCase(users, bins, qr.New(storageDir, bot.Self.UserName), photos)
	dumpUC := usecase.NewDumpReportUsecase(users, dumps, photos)
	workflowUC := usecase.NewWorkflowUseCase(workflows, bins)
	handler := telegram.New(bot, registerUC, dumpUC, workflowUC, telegram.Config{
		Admins: adminIDs,
		Collectors: os.Getenv("COLLECTOR_TELEGRAM_IDS"),
		Recyclers: os.Getenv("RECYCLER_TELEGRAM_IDS"),
		Schedule: os.Getenv("PICKUP_SCHEDULE"),
		Timezone: envOr("TIMEZONE", "Asia/Tashkent"),
		ReminderHour: intEnv("REMINDER_HOUR", 19),
		MiniAppURL: miniAppURL,
	})

	log.Println("EcoMap bot started")
	go handler.Start(ctx)
	<-ctx.Done()
	shutdownHealthServer(server)
	log.Println("EcoMap bot stopped")
}

func shutdownHealthServer(server *http.Server) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = server.Shutdown(ctx)
}

func envOr(key, fallback string) string {
	if value := os.Getenv(key); value != "" { return value }
	return fallback
}

func intEnv(key string, fallback int) int {
	value, err := strconv.Atoi(os.Getenv(key))
	if err != nil { return fallback }
	return value
}
