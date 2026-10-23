package usecase

import (
	"context"
	"errors"
	"testing"

	"ecomap/internal/domain/bin"
	"ecomap/internal/domain/dump"
	"ecomap/internal/domain/user"
	"ecomap/internal/dto"
)

type fakeUsers struct {
	byTG   map[int64]*user.User
	nextID uint64
}

func newFakeUsers() *fakeUsers { return &fakeUsers{byTG: map[int64]*user.User{}} }

func (f *fakeUsers) CreateUser(_ context.Context, u *user.User) error {
	f.nextID++
	u.ID = f.nextID
	f.byTG[u.TelegramID] = u
	return nil
}

func (f *fakeUsers) GetByTelegramID(_ context.Context, id int64) (*user.User, error) {
	return f.byTG[id], nil
}

type fakeBins struct {
	items  map[uint64]*bin.Bin
	nextID uint64
}

func newFakeBins() *fakeBins { return &fakeBins{items: map[uint64]*bin.Bin{}} }

func (f *fakeBins) Create(_ context.Context, b *bin.Bin) error {
	f.nextID++
	b.ID = f.nextID
	f.items[b.ID] = b
	return nil
}

func (f *fakeBins) GetByQR(_ context.Context, qr string) (*bin.Bin, error) {
	for _, b := range f.items {
		if b.QRCode == qr {
			return b, nil
		}
	}
	return nil, nil
}

func (f *fakeBins) GetByOwner(_ context.Context, ownerID uint64) ([]*bin.Bin, error) {
	var out []*bin.Bin
	for _, b := range f.items {
		if b.OwnerID == ownerID {
			out = append(out, b)
		}
	}
	return out, nil
}

func (f *fakeBins) Verify(_ context.Context, id uint64) error {
	f.items[id].Verified = true
	return nil
}

func (f *fakeBins) GetByID(_ context.Context, id uint64) (*bin.Bin, error) {
	return f.items[id], nil
}

func (f *fakeBins) Update(_ context.Context, b *bin.Bin) error {
	f.items[b.ID] = b
	return nil
}

type fakeDumps struct{ saved []*dump.Dump }

func (f *fakeDumps) CreateDump(_ context.Context, d *dump.Dump) error {
	f.saved = append(f.saved, d)
	return nil
}

type fakeQR struct{ n int }

func (f *fakeQR) Generate() (string, string, error) {
	f.n++
	return "code", "storage/qr/code.png", nil
}

type fakePhotos struct{ err error }

func (f *fakePhotos) SaveTelegramPhoto(_ context.Context, fileID, folder string) (string, error) {
	if f.err != nil {
		return "", f.err
	}
	return folder + "/" + fileID + ".jpg", nil
}

func TestRegister(t *testing.T) {
	ctx := context.Background()
	users, bins := newFakeUsers(), newFakeBins()
	uc := NewRegisterUserUseCase(users, bins, &fakeQR{}, &fakePhotos{})

	qrPath, err := uc.Execute(ctx, dto.RegisterUserInput{TelegramID: 100, FullName: "Ali", BinPhoto: "p1", Lat: 42.4, Lon: 59.6})
	if err != nil {
		t.Fatal(err)
	}
	if qrPath == "" {
		t.Fatal("empty qr path")
	}

	got, _ := uc.GetUserBins(ctx, 100)
	if len(got) != 1 || got[0].QRCode != "code" || got[0].Status != bin.StatusEmpty {
		t.Fatalf("unexpected bins: %+v", got)
	}

	if _, err := uc.Execute(ctx, dto.RegisterUserInput{TelegramID: 100}); !errors.Is(err, ErrAlreadyRegistered) {
		t.Fatalf("expected ErrAlreadyRegistered, got %v", err)
	}
}

func TestRegisterDoesNotCreateUserWhenPhotoFails(t *testing.T) {
	ctx := context.Background()
	users := newFakeUsers()
	uc := NewRegisterUserUseCase(users, newFakeBins(), &fakeQR{}, &fakePhotos{err: errors.New("telegram is down")})

	if _, err := uc.Execute(ctx, dto.RegisterUserInput{TelegramID: 100}); err == nil {
		t.Fatal("expected error")
	}
	if exists, _ := uc.UserExists(ctx, 100); exists {
		t.Fatal("user was created even though registration failed")
	}
}

func TestUpdateBinStatusChecksOwner(t *testing.T) {
	ctx := context.Background()
	uc := NewRegisterUserUseCase(newFakeUsers(), newFakeBins(), &fakeQR{}, &fakePhotos{})

	uc.Execute(ctx, dto.RegisterUserInput{TelegramID: 1})
	uc.Execute(ctx, dto.RegisterUserInput{TelegramID: 2})

	if err := uc.UpdateBinStatus(ctx, 2, 1, bin.StatusFull); !errors.Is(err, ErrNotOwner) {
		t.Fatalf("expected ErrNotOwner, got %v", err)
	}
	if err := uc.UpdateBinStatus(ctx, 1, 1, "hacked"); !errors.Is(err, ErrUnknownStatus) {
		t.Fatalf("expected ErrUnknownStatus, got %v", err)
	}
	if err := uc.UpdateBinStatus(ctx, 1, 99, bin.StatusFull); !errors.Is(err, ErrBinNotFound) {
		t.Fatalf("expected ErrBinNotFound, got %v", err)
	}
	if err := uc.UpdateBinStatus(ctx, 1, 1, bin.StatusFull); err != nil {
		t.Fatal(err)
	}

	bins, _ := uc.GetUserBins(ctx, 1)
	if bins[0].Status != bin.StatusFull {
		t.Fatalf("status not updated: %s", bins[0].Status)
	}
}

func TestDumpReport(t *testing.T) {
	ctx := context.Background()
	users, dumps := newFakeUsers(), &fakeDumps{}
	uc := NewDumpReportUsecase(users, dumps, &fakePhotos{})

	if err := uc.Execute(ctx, dto.DumpReportInput{TelegramID: 5}); !errors.Is(err, ErrNotRegistered) {
		t.Fatalf("expected ErrNotRegistered, got %v", err)
	}

	users.CreateUser(ctx, &user.User{TelegramID: 5})
	err := uc.Execute(ctx, dto.DumpReportInput{TelegramID: 5, PhotoID: "x", Lat: 1, Lon: 2, Description: "garbage"})
	if err != nil {
		t.Fatal(err)
	}
	if len(dumps.saved) != 1 || dumps.saved[0].PhotoURL != "dump_reports/x.jpg" {
		t.Fatalf("unexpected dumps: %+v", dumps.saved)
	}
}
