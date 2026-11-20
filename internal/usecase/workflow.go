package usecase

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"

	"ecomap/internal/domain/bin"
	"ecomap/internal/domain/workflow"
)

var (
	ErrNoOpenBatch     = errors.New("collector has no open batch")
	ErrBatchNotFound   = errors.New("batch not found")
	ErrInvalidWeight   = errors.New("weight must be between 0 and 1000 kg")
	ErrMaterialMismatch = errors.New("bin material does not match batch")
	ErrInvalidMaterial = errors.New("unsupported material")
	ErrInvalidLanguage = errors.New("unsupported language")
)

type WorkflowUseCase struct {
	repo workflow.Repository
	bins bin.BinRepository
}

func NewWorkflowUseCase(repo workflow.Repository, bins bin.BinRepository) *WorkflowUseCase {
	return &WorkflowUseCase{repo: repo, bins: bins}
}

func (uc *WorkflowUseCase) CreateIssue(ctx context.Context, telegramID int64, binID uint64, description string) (uint64, error) {
	b, err := uc.bins.GetByID(ctx, binID)
	if err != nil {
		return 0, err
	}
	if b == nil {
		return 0, ErrBinNotFound
	}
	description = strings.TrimSpace(description)
	if description == "" {
		return 0, errors.New("description is required")
	}
	return uc.repo.CreateIssue(ctx, telegramID, binID, description)
}

func (uc *WorkflowUseCase) StartBatch(ctx context.Context, collectorID int64, material string) (workflow.Batch, error) {
	if !validMaterial(material) {
		return workflow.Batch{}, ErrInvalidMaterial
	}
	if active, err := uc.repo.GetOpenBatch(ctx, collectorID); err != nil {
		return workflow.Batch{}, err
	} else if active != nil {
		return *active, nil
	}
	return uc.repo.CreateBatch(ctx, collectorID, material)
}

func (uc *WorkflowUseCase) OpenBatch(ctx context.Context, collectorID int64) (*workflow.Batch, error) {
	return uc.repo.GetOpenBatch(ctx, collectorID)
}

func (uc *WorkflowUseCase) RecordPickup(ctx context.Context, collectorID int64, qr string, weight float64) (workflow.Batch, int64, error) {
	if math.IsNaN(weight) || math.IsInf(weight, 0) || weight <= 0 || weight > 1000 {
		return workflow.Batch{}, 0, ErrInvalidWeight
	}
	batch, err := uc.repo.GetOpenBatch(ctx, collectorID)
	if err != nil {
		return workflow.Batch{}, 0, err
	}
	if batch == nil {
		return workflow.Batch{}, 0, ErrNoOpenBatch
	}
	binItem, err := uc.bins.GetByQR(ctx, qr)
	if err != nil {
		return workflow.Batch{}, 0, err
	}
	if binItem == nil {
		return workflow.Batch{}, 0, ErrBinNotFound
	}
	if binItem.Material != batch.Material {
		return workflow.Batch{}, 0, ErrMaterialMismatch
	}
	return uc.repo.RecordPickup(ctx, batch.ID, collectorID, qr, weight)
}

func (uc *WorkflowUseCase) FinishBatch(ctx context.Context, collectorID int64) (workflow.Batch, error) {
	batch, err := uc.repo.GetOpenBatch(ctx, collectorID)
	if err != nil {
		return workflow.Batch{}, err
	}
	if batch == nil {
		return workflow.Batch{}, ErrNoOpenBatch
	}
	if batch.CollectedKG <= 0 {
		return workflow.Batch{}, errors.New("batch is empty")
	}
	return uc.repo.FinishBatch(ctx, batch.ID, collectorID)
}

func (uc *WorkflowUseCase) AcceptBatch(ctx context.Context, code string, recyclerID int64, weight float64) (workflow.Batch, error) {
	if math.IsNaN(weight) || math.IsInf(weight, 0) || weight < 0 || weight > 100000 {
		return workflow.Batch{}, ErrInvalidWeight
	}
	return uc.repo.AcceptBatch(ctx, strings.TrimSpace(code), recyclerID, weight)
}

func (uc *WorkflowUseCase) Impact(ctx context.Context, telegramID int64) ([]workflow.Impact, error) {
	return uc.repo.GetImpact(ctx, telegramID)
}

func (uc *WorkflowUseCase) UserIssues(ctx context.Context, telegramID int64) ([]workflow.Issue, error) {
	return uc.repo.ListUserIssues(ctx, telegramID)
}

func (uc *WorkflowUseCase) OpenIssues(ctx context.Context) ([]workflow.Issue, error) {
	return uc.repo.ListOpenIssues(ctx)
}

func (uc *WorkflowUseCase) ResolveIssue(ctx context.Context, id uint64) error {
	return uc.repo.ResolveIssue(ctx, id)
}

func (uc *WorkflowUseCase) UserDumps(ctx context.Context, telegramID int64) ([]workflow.DumpReport, error) {
	return uc.repo.ListUserDumps(ctx, telegramID)
}

func (uc *WorkflowUseCase) OpenDumps(ctx context.Context) ([]workflow.DumpReport, error) {
	return uc.repo.ListOpenDumps(ctx)
}

func (uc *WorkflowUseCase) ResolveDump(ctx context.Context, id uint64) error {
	return uc.repo.ResolveDump(ctx, id)
}

func (uc *WorkflowUseCase) SetReminders(ctx context.Context, telegramID int64, enabled bool) error {
	return uc.repo.SetReminders(ctx, telegramID, enabled)
}

func (uc *WorkflowUseCase) ReminderRecipients(ctx context.Context) ([]int64, error) {
	return uc.repo.ListReminderRecipients(ctx)
}

func (uc *WorkflowUseCase) Language(ctx context.Context, telegramID int64) (string, error) {
	return uc.repo.GetLanguage(ctx, telegramID)
}

func (uc *WorkflowUseCase) SetLanguage(ctx context.Context, telegramID int64, language string) error {
	if language != "kaa" && language != "ru" {
		return ErrInvalidLanguage
	}
	return uc.repo.SetLanguage(ctx, telegramID, language)
}

func (uc *WorkflowUseCase) MaterialLabel(material string) string {
	switch material {
	case bin.MaterialPlastic:
		return "🧴 Пластик"
	case bin.MaterialPaper:
		return "📄 Бумага"
	case bin.MaterialGlass:
		return "🍾 Стекло"
	default:
		return fmt.Sprintf("♻️ %s", material)
	}
}

func validMaterial(material string) bool {
	return material == bin.MaterialPlastic || material == bin.MaterialPaper || material == bin.MaterialGlass
}
