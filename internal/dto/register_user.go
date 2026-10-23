package dto

type RegisterUserInput struct {
	TelegramID int64
	FullName   string
	PhotoURL   string

	BinPhoto string
	Lat      float64
	Lon      float64
	Address  string
	Landmark string
}
