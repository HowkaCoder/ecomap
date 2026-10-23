package bin

import "context"

type BinRepository interface {
	Create(ctx context.Context, b *Bin) error
	GetByQR(ctx context.Context, qr string) (*Bin, error)
	GetByOwner(ctx context.Context, ownerID uint64) ([]*Bin, error)
	Verify(ctx context.Context, binID uint64) error
	GetByID(ctx context.Context, id uint64) (*Bin, error)
	Update(ctx context.Context, b *Bin) error
}
