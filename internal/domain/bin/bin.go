package bin 


type Bin struct {
	ID 				uint64
	OwnerID			uint64
	QRCode			string 
	PhotoURL		string
	Lat 			float64
	Lon				float64
	Address			string 
	Landmark 		string 
	Verified 		bool 
	Status			string 
}