package usecase

type QRService interface {
	Generate() (filePath string, err error)
}
