package qr

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/google/uuid"
	qrcode "github.com/skip2/go-qrcode"
)

type Service struct {
	dir         string
	botUsername string
}

func New(basePath, botUsername string) *Service {
	return &Service{dir: filepath.Join(basePath, "qr"), botUsername: botUsername}
}

func (s *Service) Generate() (string, string, error) {
	if err := os.MkdirAll(s.dir, 0o755); err != nil {
		return "", "", err
	}

	code := uuid.NewString()
	path := filepath.Join(s.dir, code+".png")
	payload := code
	if s.botUsername != "" {
		payload = fmt.Sprintf("https://t.me/%s?start=pickup_%s", s.botUsername, code)
	}
	if err := qrcode.WriteFile(payload, qrcode.Medium, 256, path); err != nil {
		return "", "", err
	}
	return code, path, nil
}
