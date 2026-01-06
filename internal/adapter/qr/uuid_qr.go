package qr

import (
	"path/filepath"

	"github.com/google/uuid"
	qrcode "github.com/skip2/go-qrcode"
)

type Service struct {
	basePath string
}

func New(basePath string) *Service {
	return &Service{basePath: basePath}
}

func (s *Service) Generate() (string, error) {
	secret := uuid.NewString()

	path := filepath.Join(
		s.basePath,
		"qr",
		secret+".png",
	)

	err := qrcode.WriteFile(secret, qrcode.Medium, 256, path)
	return path, err
}
