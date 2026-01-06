package dto 

type DumpReportInput struct {
	ID 				uint64
	TelegramID		int64
	UserID			uint64
	PhotoURL		string
	Lat				float64
	Lon				float64
	Address			string 
	Landmark		string
	Description		string 
	Status			string 
}