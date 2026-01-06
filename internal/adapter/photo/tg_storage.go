package photo

import (
	"io"
	"net/http"
	"os"
	"path/filepath"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type TelegramStorage struct {
	bot      *tgbotapi.BotAPI
	basePath string
}

func New(bot *tgbotapi.BotAPI, basePath string) *TelegramStorage {
	return &TelegramStorage{
		bot:      bot,
		basePath: basePath,
	}
}

func (s *TelegramStorage) SaveTelegramPhoto(
	fileID string,
	folder string,
) (string, error) {

	// 1️⃣ Получаем файл
	file, err := s.bot.GetFile(tgbotapi.FileConfig{
		FileID: fileID,
	})
	if err != nil {
		return "", err
	}

	// 2️⃣ Формируем директорию
	dir := filepath.Join(s.basePath, folder)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}

	dstPath := filepath.Join(dir, filepath.Base(file.FilePath))

	// 3️⃣ Получаем прямой URL
	url ,  _  := s.bot.GetFileDirectURL(file.FileID)

	// 4️⃣ Скачиваем через net/http
	resp, err := http.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	// 5️⃣ Сохраняем файл
	out, err := os.Create(dstPath)
	if err != nil {
		return "", err
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	if err != nil {
		return "", err
	}

	return dstPath, nil
}
