package workflow

import "time"

type Batch struct {
	ID             uint64
	Code           string
	Material       string
	Status         string
	CollectedKG    float64
	AcceptedKG     float64
	HasAcceptedKG  bool
	CreatedAt      time.Time
}

type Issue struct {
	ID          uint64
	BinID       uint64
	Material    string
	Address     string
	Description string
	Status      string
	CreatedAt   time.Time
}

type DumpReport struct {
	ID          uint64
	Address     string
	Description string
	Status      string
	CreatedAt   time.Time
}

type Impact struct {
	Material string
	AcceptedKG float64
}
