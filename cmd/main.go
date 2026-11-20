package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/jackc/pgx/v5/pgxpool"

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
	server := &http.Server{Addr: ":" + port, Handler: healthHandler(db, botToken), ReadHeaderTimeout: 5 * time.Second}
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

	users := postgres.NewUserRepo(db)
	bins := postgres.NewBinRepo(db)
	dumps := postgres.NewDumpRepo(db)
	workflows := postgres.NewWorkflowRepo(db)
	photos := photo.New(bot, storageDir)

	registerUC := usecase.NewRegisterUserUseCase(users, bins, qr.New(storageDir, bot.Self.UserName), photos)
	dumpUC := usecase.NewDumpReportUsecase(users, dumps, photos)
	workflowUC := usecase.NewWorkflowUseCase(workflows, bins)
	handler := telegram.New(bot, registerUC, dumpUC, workflowUC, telegram.Config{
		Admins: os.Getenv("ADMIN_TELEGRAM_IDS"),
		Collectors: os.Getenv("COLLECTOR_TELEGRAM_IDS"),
		Recyclers: os.Getenv("RECYCLER_TELEGRAM_IDS"),
		Schedule: os.Getenv("PICKUP_SCHEDULE"),
		Timezone: envOr("TIMEZONE", "Asia/Tashkent"),
		ReminderHour: intEnv("REMINDER_HOUR", 19),
	})

	log.Println("EcoMap bot started")
	go handler.Start(ctx)
	<-ctx.Done()
	shutdownHealthServer(server)
	log.Println("EcoMap bot stopped")
}

func healthHandler(db *pgxpool.Pool, botToken string) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		status := "ok"
		if botToken == "" { status = "waiting_for_bot_token" }
		if err := db.Ping(ctx); err != nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			_ = json.NewEncoder(w).Encode(map[string]string{"status": "database_unavailable"})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]string{"status": status})
	})
	mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(`<!doctype html><html><meta name="viewport" content="width=device-width,initial-scale=1"><body style="margin:0;background:#f4f0e5;color:#14372d;font:16px system-ui"><main style="max-width:720px;margin:12vh auto;padding:32px"><p style="color:#2f6b53;font-weight:700">ECOMAP · BIN4HOME PILOT</p><h1>Test environment is ready</h1><p>This service runs the Telegram bot and exposes a database health check at <code>/healthz</code>.</p></main></body></html>`))
	})
	return mux
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
