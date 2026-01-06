package dump 


import "context"

type DumpRepository interface {

	CreateDump(ctx context.Context , dump *Dump) error 
	
}