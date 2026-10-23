package usecase

import (
	"context"

	"ecomap/internal/domain/bin"
	"ecomap/internal/domain/user"
	"ecomap/internal/dto"
)

type RegisterUserUseCase struct {
	users  user.UserRepository
	bins   bin.BinRepository
	qr     QRService
	photos PhotoStorage
}

func NewRegisterUserUseCase(users user.UserRepository, bins bin.BinRepository, qr QRService, photos PhotoStorage) *RegisterUserUseCase {
	return &RegisterUserUseCase{users: users, bins: bins, qr: qr, photos: photos}
}

func (uc *RegisterUserUseCase) UserExists(ctx context.Context, telegramID int64) (bool, error) {
	u, err := uc.users.GetByTelegramID(ctx, telegramID)
	if err != nil {
		return false, err
	}
	return u != nil, nil
}

func (uc *RegisterUserUseCase) Execute(ctx context.Context, input dto.RegisterUserInput) (string, error) {
	existing, err := uc.users.GetByTelegramID(ctx, input.TelegramID)
	if err != nil {
		return "", err
	}
	if existing != nil {
		return "", ErrAlreadyRegistered
	}

	photoPath, err := uc.photos.SaveTelegramPhoto(ctx, input.BinPhoto, "bins")
	if err != nil {
		return "", err
	}

	code, qrPath, err := uc.qr.Generate()
	if err != nil {
		return "", err
	}

	u := &user.User{
		TelegramID: input.TelegramID,
		FullName:   input.FullName,
		PhotoURL:   input.PhotoURL,
	}
	if err := uc.users.CreateUser(ctx, u); err != nil {
		return "", err
	}

	b := &bin.Bin{
		OwnerID:  u.ID,
		QRCode:   code,
		PhotoURL: photoPath,
		Lat:      input.Lat,
		Lon:      input.Lon,
		Address:  input.Address,
		Landmark: input.Landmark,
		Status:   bin.StatusEmpty,
	}
	if err := uc.bins.Create(ctx, b); err != nil {
		return "", err
	}

	return qrPath, nil
}

func (uc *RegisterUserUseCase) GetUserBins(ctx context.Context, telegramID int64) ([]*bin.Bin, error) {
	u, err := uc.users.GetByTelegramID(ctx, telegramID)
	if err != nil {
		return nil, err
	}
	if u == nil {
		return nil, ErrNotRegistered
	}
	return uc.bins.GetByOwner(ctx, u.ID)
}

func (uc *RegisterUserUseCase) UpdateBinStatus(ctx context.Context, telegramID int64, binID uint64, status string) error {
	if status != bin.StatusEmpty && status != bin.StatusFull {
		return ErrUnknownStatus
	}

	u, err := uc.users.GetByTelegramID(ctx, telegramID)
	if err != nil {
		return err
	}
	if u == nil {
		return ErrNotRegistered
	}

	b, err := uc.bins.GetByID(ctx, binID)
	if err != nil {
		return err
	}
	if b == nil {
		return ErrBinNotFound
	}
	if b.OwnerID != u.ID {
		return ErrNotOwner
	}

	b.Status = status
	return uc.bins.Update(ctx, b)
}
