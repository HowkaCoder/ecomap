package dto

type DumpReportInput struct {
	TelegramID  int64
	PhotoID     string
	Lat         float64
	Lon         float64
	Address     string
	Landmark    string
	Description string
	Status      string
}
