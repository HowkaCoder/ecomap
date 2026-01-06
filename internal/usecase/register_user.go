package usecase 

import (
	"errors"

	"ecomap/internal/domain/bin"
    "context"
	"ecomap/internal/dto"
	"log"
	"ecomap/internal/domain/user"
)

type RegisterUserUseCase struct {
    users user.UserRepository
    bins  bin.BinRepository
    qr    QRService
	photos PhotoStorage
}

func (uc *RegisterUserUseCase) UserExists(
	ctx context.Context,
	telegramID int64,
) (bool, error) {
	u, err := uc.users.GetByTelegramID(ctx, int64(telegramID))
	if err != nil {
		return false, err
	}
	return u != nil, nil
}

func (uc *RegisterUserUseCase) GetUserBins(
	ctx context.Context,
	telegramID int64,
) ([]*bin.Bin, error) {

	user, err := uc.users.GetByTelegramID(ctx, telegramID)
	if err != nil {
		return nil, err
	}

	if user == nil {
		return nil, errors.New("user not registered")
	}

	return uc.bins.GetByOwner(ctx, int64(user.ID))
}

func (uc *RegisterUserUseCase) UpdateBinStatus(ctx context.Context, binID int64, status string) error {
	log.Println("bin status is updating here")
	b, err := uc.bins.GetByID(ctx, binID)
	log.Println(err)
	if err != nil {
		return err
	}
	b.Status = status
	log.Println(b)
	return uc.bins.Update(ctx, b)
}


func NewRegisterUserUseCase(users user.UserRepository,bins bin.BinRepository,qr QRService , photo PhotoStorage) *RegisterUserUseCase {
    return &RegisterUserUseCase{users, bins, qr , photo}
}

func (uc *RegisterUserUseCase) Execute(
	ctx context.Context,
	input dto.RegisterUserInput,
) (string, error) {

	existing, _ := uc.users.GetByTelegramID(ctx, input.TelegramID)
	if existing != nil {
		return "", errors.New("user already exists")
	}

	user := &user.User{
		TelegramID: input.TelegramID,
		FullName:   input.FullName,
		PhotoURL:  input.PhotoURL,
	}

	if err := uc.users.CreateUser(ctx, user); err != nil {
		return "", err
	}

	qrPath, err := uc.qr.Generate()
	if err != nil {
		return "", err
	}

	photoPath, err := uc.photos.SaveTelegramPhoto(
		input.BinPhoto,
		"bins",
	)
	if err != nil {
		return "", err
	}

	bin := &bin.Bin{
		OwnerID:  user.ID,
		QRCode:   qrPath,
		PhotoURL: photoPath,
		Lat:      input.Lat,
		Lon:      input.Lon,
	}

	if err := uc.bins.Create(ctx, bin); err != nil {
		return "", err
	}

	return qrPath, nil
}
