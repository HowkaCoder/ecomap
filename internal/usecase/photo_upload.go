package usecase

type PhotoStorage interface {
	SaveTelegramPhoto(fileID string, folder string) (string, error)
}
