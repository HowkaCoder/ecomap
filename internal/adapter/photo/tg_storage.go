package photo

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/google/uuid"
)

type TelegramStorage struct {
	bot      *tgbotapi.BotAPI
	basePath string
	client   *http.Client
}

func New(bot *tgbotapi.BotAPI, basePath string) *TelegramStorage {
	return &TelegramStorage{
		bot:      bot,
		basePath: basePath,
		client:   &http.Client{Timeout: 30 * time.Second},
	}
}

func (s *TelegramStorage) SaveTelegramPhoto(ctx context.Context, fileID, folder string) (string, error) {
	url, err := s.bot.GetFileDirectURL(fileID)
	if err != nil {
		return "", err
	}

	dir := filepath.Join(s.basePath, folder)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download photo: %s", resp.Status)
	}

	dst := filepath.Join(dir, uuid.NewString()+".jpg")
	out, err := os.Create(dst)
	if err != nil {
		return "", err
	}
	defer out.Close()

	if _, err := io.Copy(out, resp.Body); err != nil {
		os.Remove(dst)
		return "", err
	}
	return dst, nil
}
