package seed

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/iag/crm/backend/internal/store"
)

// EnsureJourneySteps seeds default journey steps when missing (safe on every startup).
func EnsureJourneySteps(ctx context.Context, pool *pgxpool.Pool) error {
	repo := store.New(pool)
	if err := repo.SeedJourneyStepsIfEmpty(ctx, "JRN-001", defaultJourney001Steps()); err != nil {
		return err
	}
	return repo.SeedJourneyStepsIfEmpty(ctx, "JRN-002", defaultJourney002Steps())
}

func defaultJourney001Steps() []store.JourneyStep {
	return []store.JourneyStep{
		{ID: "JST-001", JourneyID: "JRN-001", StepOrder: 1, StepType: "email", Title: "Welcome email",
			Config: map[string]any{"subject": "Welcome to IAG Coffee", "body": "Thanks for connecting."}, DelayHours: 0},
		{ID: "JST-002", JourneyID: "JRN-001", StepOrder: 2, StepType: "wait", Title: "Wait 72h", DelayHours: 72},
		{ID: "JST-003", JourneyID: "JRN-001", StepOrder: 3, StepType: "activity", Title: "Schedule cupping",
			Config: map[string]any{"subject": "Schedule cupping call"}, DelayHours: 0},
		{ID: "JST-004", JourneyID: "JRN-001", StepOrder: 4, StepType: "email", Title: "Sample follow-up",
			Config: map[string]any{"subject": "Your sample request"}, DelayHours: 168},
		{ID: "JST-005", JourneyID: "JRN-001", StepOrder: 5, StepType: "score", Title: "Boost engagement",
			Config: map[string]any{"delta": 8}, DelayHours: 0},
	}
}

func defaultJourney002Steps() []store.JourneyStep {
	return []store.JourneyStep{
		{ID: "JST-101", JourneyID: "JRN-002", StepOrder: 1, StepType: "email", Title: "Outlet welcome",
			Config: map[string]any{"subject": "Welcome outlet partner"}, DelayHours: 0},
		{ID: "JST-102", JourneyID: "JRN-002", StepOrder: 2, StepType: "wait", Title: "Wait 48h", DelayHours: 48},
		{ID: "JST-103", JourneyID: "JRN-002", StepOrder: 3, StepType: "activity", Title: "First order check",
			Config: map[string]any{"subject": "Confirm first order"}, DelayHours: 0},
	}
}
