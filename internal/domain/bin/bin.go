package bin

const (
	StatusEmpty = "Пустая"
	StatusFull  = "Нужна уборка"
	MaterialPlastic = "plastic"
	MaterialPaper   = "paper"
	MaterialGlass   = "glass"
	MaterialMixed   = "mixed"
)

type Bin struct {
	ID       uint64
	OwnerID  uint64
	QRCode   string
	PhotoURL string
	Lat      float64
	Lon      float64
	Address  string
	Landmark string
	Verified bool
	Status   string
	Material string
}
