package workflow

import "context"

type Repository interface {
	CreateIssue(context.Context, int64, uint64, string) (uint64, error)
	ListUserIssues(context.Context, int64) ([]Issue, error)
	ListOpenIssues(context.Context) ([]Issue, error)
	ResolveIssue(context.Context, uint64) error
	CreateBatch(context.Context, int64, string) (Batch, error)
	GetOpenBatch(context.Context, int64) (*Batch, error)
	RecordPickup(context.Context, uint64, int64, string, float64) (Batch, int64, error)
	FinishBatch(context.Context, uint64, int64) (Batch, error)
	AcceptBatch(context.Context, string, int64, float64) (Batch, error)
	GetImpact(context.Context, int64) ([]Impact, error)
	ListUserDumps(context.Context, int64) ([]DumpReport, error)
	ListOpenDumps(context.Context) ([]DumpReport, error)
	ResolveDump(context.Context, uint64) error
	SetReminders(context.Context, int64, bool) error
	ListReminderRecipients(context.Context) ([]int64, error)
}
