package usecase

type QRService interface {
	Generate() (code string, filePath string, err error)
}
