package usecase 

import (
	"errors"

	"ecomap/internal/domain/dump"
    "context"
	"ecomap/internal/dto"
	"ecomap/internal/domain/user"
)

type DumpReportUsecase struct {
	users 	user.UserRepository
	dumps 	dump.DumpRepository
	photos  PhotoStorage
}

func NewDumpReportUsecase(users user.UserRepository , dumps dump.DumpRepository , photo PhotoStorage) *DumpReportUsecase {
	return &DumpReportUsecase{users:users , dumps:dumps , photos:photo}
}

func (dc *DumpReportUsecase) Execute(ctx context.Context , input dto.DumpReportInput) error {
	
	existing, _ := dc.users.GetByTelegramID(ctx, input.TelegramID)
    
	if existing == nil {
        return errors.New("user doesnt exists")
    }

	photoPath, err := dc.photos.SaveTelegramPhoto(
		input.PhotoURL,
		"dump_reports",
	)
	if err != nil {
		return err
	}


	d1 := &dump.Dump{
		UserID: 	existing.ID, 
		PhotoURL: 	photoPath,
		Lat:		input.Lat,
		Lon:		input.Lon,
        Address:  	input.Address,
        Landmark: 	input.Landmark,
		Description:		input.Description,
		Status:		input.Status,
	}
	return dc.dumps.CreateDump(ctx , d1)

}