package usecase

import (
	"context"

	"ecomap/internal/domain/dump"
	"ecomap/internal/domain/user"
	"ecomap/internal/dto"
)

type DumpReportUsecase struct {
	users  user.UserRepository
	dumps  dump.DumpRepository
	photos PhotoStorage
}

func NewDumpReportUsecase(users user.UserRepository, dumps dump.DumpRepository, photos PhotoStorage) *DumpReportUsecase {
	return &DumpReportUsecase{users: users, dumps: dumps, photos: photos}
}

func (uc *DumpReportUsecase) Execute(ctx context.Context, input dto.DumpReportInput) error {
	u, err := uc.users.GetByTelegramID(ctx, input.TelegramID)
	if err != nil {
		return err
	}
	if u == nil {
		return ErrNotRegistered
	}

	photoPath, err := uc.photos.SaveTelegramPhoto(ctx, input.PhotoID, "dump_reports")
	if err != nil {
		return err
	}

	return uc.dumps.CreateDump(ctx, &dump.Dump{
		UserID:      u.ID,
		PhotoURL:    photoPath,
		Lat:         input.Lat,
		Lon:         input.Lon,
		Address:     input.Address,
		Landmark:    input.Landmark,
		Description: input.Description,
		Status:      input.Status,
	})
}
