package qr

import (
	"os"
	"path/filepath"

	"github.com/google/uuid"
	qrcode "github.com/skip2/go-qrcode"
)

type Service struct {
	dir string
}

func New(basePath string) *Service {
	return &Service{dir: filepath.Join(basePath, "qr")}
}

func (s *Service) Generate() (string, string, error) {
	if err := os.MkdirAll(s.dir, 0o755); err != nil {
		return "", "", err
	}

	code := uuid.NewString()
	path := filepath.Join(s.dir, code+".png")
	if err := qrcode.WriteFile(code, qrcode.Medium, 256, path); err != nil {
		return "", "", err
	}
	return code, path, nil
}
