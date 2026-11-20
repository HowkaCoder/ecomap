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

func (uc *DumpReportUsecase) Execute(ctx context.Context, input dto.DumpReportInput) (uint64, error) {
	u, err := uc.users.GetByTelegramID(ctx, input.TelegramID)
	if err != nil {
		return 0, err
	}
	if u == nil {
		return 0, ErrNotRegistered
	}

	photoPath, err := uc.photos.SaveTelegramPhoto(ctx, input.PhotoID, "dump_reports")
	if err != nil {
		return 0, err
	}

	report := &dump.Dump{
		UserID:      u.ID,
		PhotoURL:    photoPath,
		Lat:         input.Lat,
		Lon:         input.Lon,
		Address:     input.Address,
		Landmark:    input.Landmark,
		Description: input.Description,
		Status:      input.Status,
	}
	if err := uc.dumps.CreateDump(ctx, report); err != nil {
		return 0, err
	}
	return report.ID, nil
}
