package usecase

import "context"

type PhotoStorage interface {
	SaveTelegramPhoto(ctx context.Context, fileID, folder string) (string, error)
}
