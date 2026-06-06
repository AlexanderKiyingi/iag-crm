// Package journey runs multi-step CRM journey automations.
package journey

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/iag/crm/backend/internal/events"
	"github.com/iag/crm/backend/internal/store"
)

// Runner executes due journey enrollments on a timer.
type Runner struct {
	Repo   *store.Repository
	Events eventsPublisher
	Tick   time.Duration
	Batch  int
}

type eventsPublisher interface {
	PublishCommercial(ctx context.Context, eventType string, data map[string]any, key string)
}

// Run blocks until ctx is cancelled, ticking every interval.
func (r *Runner) Run(ctx context.Context) {
	if r == nil || r.Repo == nil {
		return
	}
	tick := r.Tick
	if tick <= 0 {
		tick = 30 * time.Second
	}
	batch := r.Batch
	if batch <= 0 {
		batch = 32
	}
	ticker := time.NewTicker(tick)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := r.TickOnce(ctx, batch); err != nil {
				slog.Warn("journey runner tick", "err", err)
			}
		}
	}
}

// TickOnce processes up to limit due enrollments.
func (r *Runner) TickOnce(ctx context.Context, limit int) error {
	due, err := r.Repo.ClaimDueEnrollments(ctx, limit)
	if err != nil {
		return err
	}
	for _, en := range due {
		if err := r.processEnrollment(ctx, en); err != nil {
			slog.Warn("journey enrollment", "id", en.ID, "err", err)
		}
	}
	return nil
}

func (r *Runner) processEnrollment(ctx context.Context, en store.JourneyEnrollment) error {
	step, err := r.Repo.GetJourneyStepAtOrder(ctx, en.JourneyID, en.CurrentStep)
	if err != nil {
		// No step at current order — journey complete
		_ = r.Repo.AdvanceEnrollment(ctx, en.ID, en.CurrentStep, 0, true)
		_ = r.Repo.BumpJourneyConversion(ctx, en.JourneyID)
		if r.Events != nil {
			r.Events.PublishCommercial(ctx, events.TypeJourneyCompleted, map[string]any{
				"enrollment_id": en.ID, "journey_id": en.JourneyID,
			}, en.ID)
		}
		return nil
	}

	detail, execErr := r.executeStep(ctx, en, step)
	status := "ok"
	if execErr != nil {
		status = "error"
		detail = execErr.Error()
	}
	_ = r.Repo.LogJourneyStep(ctx, en.ID, step.ID, step.StepOrder, step.StepType, status, detail)

	nextOrder := en.CurrentStep + 1
	nextStep, nextErr := r.Repo.GetJourneyStepAtOrder(ctx, en.JourneyID, nextOrder)
	if nextErr != nil {
		_ = r.Repo.AdvanceEnrollment(ctx, en.ID, nextOrder, 0, true)
		_ = r.Repo.BumpJourneyConversion(ctx, en.JourneyID)
		if r.Events != nil {
			r.Events.PublishCommercial(ctx, events.TypeJourneyCompleted, map[string]any{
				"enrollment_id": en.ID, "journey_id": en.JourneyID,
			}, en.ID)
		}
		return execErr
	}
	return r.Repo.AdvanceEnrollment(ctx, en.ID, nextOrder, nextStep.DelayHours, false)
}

func (r *Runner) executeStep(ctx context.Context, en store.JourneyEnrollment, step store.JourneyStep) (string, error) {
	switch strings.ToLower(step.StepType) {
	case "email":
		subject := strConfig(step.Config, "subject", "Journey: "+step.Title)
		body := strConfig(step.Config, "body", "Automated journey email for "+en.SubjectName)
		_, err := r.Repo.CreateEmailSend(ctx, map[string]any{
			"subject": subject, "template": step.Title, "body": body, "status": "queued",
		})
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("queued email to %s", en.SubjectEmail), nil
	case "activity":
		subject := strConfig(step.Config, "subject", step.Title)
		body := strConfig(step.Config, "body", "")
		_, err := r.Repo.CreateActivity(ctx, store.ActivityInput{
			Type:      "task",
			Subject:   subject,
			Body:      body,
			Account:   "",
			ContactID: en.ContactID,
			DealID:    "",
			Owner:     "journey-runner",
		})
		if err != nil {
			return "", err
		}
		return "activity created", nil
	case "wait":
		return "wait elapsed", nil
	case "score":
		if en.LeadID != "" {
			delta := intNumConfig(step.Config, "delta", 5)
			_ = r.Repo.BumpLeadScore(ctx, en.LeadID, delta)
			return fmt.Sprintf("lead score +%d", delta), nil
		}
		return "no lead to score", nil
	default:
		return "unknown step type " + step.StepType, nil
	}
}

// AutoEnrollContact enrolls a new contact into journeys matching trigger hints.
func AutoEnrollContact(ctx context.Context, repo *store.Repository, contactID, email, name string) error {
	ids, err := repo.ListActiveJourneysByTrigger(ctx, "new contact")
	if err != nil {
		return err
	}
	for _, jid := range ids {
		if _, err := repo.EnrollInJourney(ctx, jid, store.EnrollInput{
			ContactID: contactID, SubjectEmail: email, SubjectName: name,
		}); err != nil {
			return err
		}
	}
	return nil
}

func strConfig(cfg map[string]any, key, fallback string) string {
	if cfg == nil {
		return fallback
	}
	if v, ok := cfg[key].(string); ok && strings.TrimSpace(v) != "" {
		return v
	}
	return fallback
}

func intNumConfig(cfg map[string]any, key string, fallback int) int {
	if cfg == nil {
		return fallback
	}
	switch v := cfg[key].(type) {
	case float64:
		return int(v)
	case int:
		return v
	default:
		return fallback
	}
}
