package webapp

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io/fs"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed static/*
var assets embed.FS

type App struct {
	db         *pgxpool.Pool
	botToken   string
	admins     map[int64]bool
	schedule   string
	timezone   string
}

type telegramUser struct {
	ID int64 `json:"id"`
}

type identity struct {
	ID       int64
	Username string
}

type DailyStat struct {
	Day string  `json:"day"`
	KG  float64 `json:"kg"`
}

type MaterialStat struct {
	Material string  `json:"material"`
	KG       float64 `json:"kg"`
}

type ResidentBin struct {
	ID       uint64 `json:"id"`
	Material string `json:"material"`
	Status   string `json:"status"`
	Address  string `json:"address"`
}

type ResidentReport struct {
	ID          uint64    `json:"id"`
	Kind        string    `json:"kind"`
	Address     string    `json:"address"`
	Description string    `json:"description"`
	Status      string    `json:"status"`
	CreatedAt   time.Time `json:"createdAt"`
}

type AdminStats struct {
	Residents         int            `json:"residents"`
	Bins              int            `json:"bins"`
	FullBins          int            `json:"fullBins"`
	DumpReports       int            `json:"dumpReports"`
	OpenDumpReports   int            `json:"openDumpReports"`
	OpenPickupIssues  int            `json:"openPickupIssues"`
	CollectionBatches int            `json:"collectionBatches"`
	CollectedKG       float64        `json:"collectedKg"`
	AcceptedKG        float64        `json:"acceptedKg"`
	Daily             []DailyStat    `json:"daily"`
	Materials         []MaterialStat `json:"materials"`
	Schedule          string         `json:"schedule"`
	Timezone          string         `json:"timezone"`
	GeneratedAt       time.Time      `json:"generatedAt"`
}

func New(db *pgxpool.Pool, botToken, adminIDs, schedule, timezone string) http.Handler {
	admins := make(map[int64]bool)
	for _, raw := range strings.Split(adminIDs, ",") {
		id, err := strconv.ParseInt(strings.TrimSpace(raw), 10, 64)
		if err == nil && id > 0 {
			admins[id] = true
		}
	}
	if timezone == "" {
		timezone = "Asia/Tashkent"
	}
	a := &App{db: db, botToken: botToken, admins: admins, schedule: schedule, timezone: timezone}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", a.health)
	mux.HandleFunc("GET /", a.home)
	mux.HandleFunc("GET /miniapp", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/miniapp/", http.StatusTemporaryRedirect)
	})
	mux.HandleFunc("GET /miniapp/", a.serveMiniApp)
	mux.HandleFunc("GET /api/miniapp/summary", a.withIdentity(a.summary))
	mux.HandleFunc("GET /api/miniapp/admin/stats", a.withIdentity(a.adminStats))
	mux.HandleFunc("POST /api/miniapp/language", a.withIdentity(a.setLanguage))
	return securityHeaders(mux)
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		next.ServeHTTP(w, r)
	})
}

func (a *App) home(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/miniapp/", http.StatusTemporaryRedirect)
}

func (a *App) serveMiniApp(w http.ResponseWriter, r *http.Request) {
	file, err := fs.ReadFile(assets, "static/index.html")
	if err != nil {
		http.Error(w, "Mini App asset unavailable", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	_, _ = w.Write(file)
}

func (a *App) health(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()
	if err := a.db.Ping(ctx); err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "database_unavailable"})
		return
	}
	status := "ok"
	if a.botToken == "" {
		status = "waiting_for_bot_token"
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": status})
}

func (a *App) withIdentity(next func(http.ResponseWriter, *http.Request, identity)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, err := a.verifyInitData(r.Header.Get("X-Telegram-Init-Data"))
		if err != nil {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "telegram_auth_required"})
			return
		}
		next(w, r, user)
	}
}

func (a *App) verifyInitData(raw string) (identity, error) {
	if raw == "" || a.botToken == "" {
		return identity{}, errors.New("missing init data or bot token")
	}
	values, err := url.ParseQuery(raw)
	if err != nil {
		return identity{}, err
	}
	receivedHash := values.Get("hash")
	if receivedHash == "" {
		return identity{}, errors.New("missing hash")
	}
	values.Del("hash")
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, key+"="+values.Get(key))
	}
	secretMAC := hmac.New(sha256.New, []byte("WebAppData"))
	_, _ = secretMAC.Write([]byte(a.botToken))
	checkMAC := hmac.New(sha256.New, secretMAC.Sum(nil))
	_, _ = checkMAC.Write([]byte(strings.Join(parts, "\n")))
	expectedHash := checkMAC.Sum(nil)
	providedHash, err := hex.DecodeString(receivedHash)
	if err != nil || len(providedHash) != len(expectedHash) || subtle.ConstantTimeCompare(providedHash, expectedHash) != 1 {
		return identity{}, errors.New("invalid hash")
	}
	authDate, err := strconv.ParseInt(values.Get("auth_date"), 10, 64)
	if err != nil {
		return identity{}, errors.New("invalid auth_date")
	}
	createdAt := time.Unix(authDate, 0)
	if time.Since(createdAt) > 24*time.Hour || createdAt.After(time.Now().Add(5*time.Minute)) {
		return identity{}, errors.New("stale init data")
	}
	var tgUser telegramUser
	if err := json.Unmarshal([]byte(values.Get("user")), &tgUser); err != nil || tgUser.ID <= 0 {
		return identity{}, errors.New("invalid telegram user")
	}
	return identity{ID: tgUser.ID}, nil
}

func (a *App) summary(w http.ResponseWriter, r *http.Request, user identity) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	var result struct {
		TelegramID  int64   `json:"telegramId"`
		Registered  bool    `json:"registered"`
		Name        string  `json:"name"`
		Language    string  `json:"language"`
		Bins        int     `json:"bins"`
		FullBins    int     `json:"fullBins"`
		AcceptedKG  float64 `json:"acceptedKg"`
		IsAdmin     bool    `json:"isAdmin"`
		Schedule    string  `json:"schedule"`
		Timezone    string  `json:"timezone"`
		ResidentBins []ResidentBin `json:"residentBins"`
		Reports []ResidentReport `json:"reports"`
	}
	result.TelegramID, result.IsAdmin = user.ID, a.admins[user.ID]
	result.Language, result.Schedule, result.Timezone = "kaa", a.schedule, a.timezone
	if err := a.db.QueryRow(ctx, `
		SELECT u.full_name, COALESCE(p.language, 'kaa')
		FROM users u LEFT JOIN user_preferences p ON p.telegram_id = u.telegram_id
		WHERE u.telegram_id = $1
	`, user.ID).Scan(&result.Name, &result.Language); err == nil {
		result.Registered = true
	} else {
		_ = a.db.QueryRow(ctx, `SELECT COALESCE((SELECT language FROM user_preferences WHERE telegram_id = $1), 'kaa')`, user.ID).Scan(&result.Language)
	}
	if result.Registered {
		if err := a.db.QueryRow(ctx, `
			SELECT count(*), count(*) FILTER (WHERE status = $2)
			FROM bins b JOIN users u ON u.id = b.owner_id WHERE u.telegram_id = $1
		`, user.ID, "Нужна уборка").Scan(&result.Bins, &result.FullBins); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "summary_query_failed"})
			return
		}
		rows, err := a.db.Query(ctx, `
			SELECT b.id, b.material, b.status, b.address
			FROM bins b JOIN users u ON u.id = b.owner_id
			WHERE u.telegram_id = $1 ORDER BY b.id
		`, user.ID)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "bins_query_failed"})
			return
		}
		for rows.Next() {
			var item ResidentBin
			if err := rows.Scan(&item.ID, &item.Material, &item.Status, &item.Address); err != nil {
				rows.Close()
				writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "bins_scan_failed"})
				return
			}
			result.ResidentBins = append(result.ResidentBins, item)
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "bins_query_failed"})
			return
		}
		rows.Close()
		rows, err = a.db.Query(ctx, `
			SELECT id, kind, address, description, status, created_at FROM (
				SELECT i.id, 'pickup'::text AS kind, b.address, i.description, i.status, i.created_at
				FROM pickup_issues i JOIN users u ON u.id = i.user_id JOIN bins b ON b.id = i.bin_id
				WHERE u.telegram_id = $1
				UNION ALL
				SELECT d.id, 'dump'::text AS kind, d.address, d.description, d.workflow_status, d.created_at
				FROM dumps d JOIN users u ON u.id = d.user_id WHERE u.telegram_id = $1
			) reports ORDER BY created_at DESC LIMIT 10
		`, user.ID)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "reports_query_failed"})
			return
		}
		for rows.Next() {
			var item ResidentReport
			if err := rows.Scan(&item.ID, &item.Kind, &item.Address, &item.Description, &item.Status, &item.CreatedAt); err != nil {
				rows.Close()
				writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "reports_scan_failed"})
				return
			}
			result.Reports = append(result.Reports, item)
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "reports_query_failed"})
			return
		}
		rows.Close()
		if err := a.db.QueryRow(ctx, `
			SELECT COALESCE(SUM(e.weight_kg * cb.accepted_weight_kg / NULLIF(cb.collected_weight_kg, 0)), 0)::float8
			FROM users u JOIN bins b ON b.owner_id = u.id
			JOIN collection_events e ON e.bin_id = b.id JOIN collection_batches cb ON cb.id = e.batch_id
			WHERE u.telegram_id = $1 AND cb.accepted_weight_kg IS NOT NULL
		`, user.ID).Scan(&result.AcceptedKG); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "impact_query_failed"})
			return
		}
	}
	writeJSON(w, http.StatusOK, result)
}

func (a *App) setLanguage(w http.ResponseWriter, r *http.Request, user identity) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	var body struct {
		Language string `json:"language"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1024)).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_request"})
		return
	}
	if body.Language != "kaa" && body.Language != "ru" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "unsupported_language"})
		return
	}
	if _, err := a.db.Exec(ctx, `
		INSERT INTO user_preferences (telegram_id, language, updated_at) VALUES ($1, $2, now())
		ON CONFLICT (telegram_id) DO UPDATE SET language = EXCLUDED.language, updated_at = now()
	`, user.ID, body.Language); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "language_update_failed"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"language": body.Language})
}

func (a *App) adminStats(w http.ResponseWriter, r *http.Request, user identity) {
	if !a.admins[user.ID] {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "admin_only"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 8*time.Second)
	defer cancel()
	stats := AdminStats{Schedule: a.schedule, Timezone: a.timezone, GeneratedAt: time.Now().UTC()}
	queries := []struct {
		sql string
		dst *int
	}{
		{`SELECT count(*) FROM users`, &stats.Residents},
		{`SELECT count(*) FROM bins`, &stats.Bins},
		{`SELECT count(*) FROM bins WHERE status = 'Нужна уборка'`, &stats.FullBins},
		{`SELECT count(*) FROM dumps`, &stats.DumpReports},
		{`SELECT count(*) FROM dumps WHERE workflow_status <> 'resolved'`, &stats.OpenDumpReports},
		{`SELECT count(*) FROM pickup_issues WHERE status <> 'resolved'`, &stats.OpenPickupIssues},
		{`SELECT count(*) FROM collection_batches`, &stats.CollectionBatches},
	}
	for _, query := range queries {
		if err := a.db.QueryRow(ctx, query.sql).Scan(query.dst); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "statistics_query_failed"})
			return
		}
	}
	if err := a.db.QueryRow(ctx, `
		SELECT COALESCE(SUM(collected_weight_kg), 0)::float8,
		       COALESCE(SUM(accepted_weight_kg), 0)::float8
		FROM collection_batches
	`).Scan(&stats.CollectedKG, &stats.AcceptedKG); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "weight_query_failed"})
		return
	}
	rows, err := a.db.Query(ctx, `
		SELECT to_char(days.day, 'Dy'), COALESCE(SUM(e.weight_kg), 0)::float8
		FROM generate_series(current_date - 6, current_date, interval '1 day') AS days(day)
		LEFT JOIN collection_events e ON e.collected_at::date = days.day::date
		GROUP BY days.day ORDER BY days.day
	`)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "daily_query_failed"})
		return
	}
	for rows.Next() {
		var item DailyStat
		if err := rows.Scan(&item.Day, &item.KG); err != nil {
			rows.Close()
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "daily_scan_failed"})
			return
		}
		stats.Daily = append(stats.Daily, item)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "daily_query_failed"})
		return
	}
	rows.Close()
	rows, err = a.db.Query(ctx, `
		SELECT material, COALESCE(SUM(accepted_weight_kg), 0)::float8
		FROM collection_batches WHERE accepted_weight_kg IS NOT NULL
		GROUP BY material ORDER BY material
	`)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "materials_query_failed"})
		return
	}
	for rows.Next() {
		var item MaterialStat
		if err := rows.Scan(&item.Material, &item.KG); err != nil {
			rows.Close()
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "materials_scan_failed"})
			return
		}
		stats.Materials = append(stats.Materials, item)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "materials_query_failed"})
		return
	}
	rows.Close()
	writeJSON(w, http.StatusOK, stats)
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

