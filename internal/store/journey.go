package store

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

// JourneyStep is one node in a journey automation.
type JourneyStep struct {
	ID         string         `json:"id"`
	JourneyID  string         `json:"journey_id"`
	StepOrder  int            `json:"step_order"`
	StepType   string         `json:"step_type"`
	Title      string         `json:"title"`
	Config     map[string]any `json:"config"`
	DelayHours int            `json:"delay_hours"`
}

// JourneyEnrollment tracks a contact/lead progressing through a journey.
type JourneyEnrollment struct {
	ID           string    `json:"id"`
	JourneyID    string    `json:"journey_id"`
	ContactID    string    `json:"contact_id,omitempty"`
	LeadID       string    `json:"lead_id,omitempty"`
	SubjectEmail string    `json:"subject_email"`
	SubjectName  string    `json:"subject_name"`
	Status       string    `json:"status"`
	CurrentStep  int       `json:"current_step"`
	NextRunAt    time.Time `json:"next_run_at"`
	EnrolledAt   time.Time `json:"enrolled_at"`
}

func (r *Repository) ListJourneySteps(ctx context.Context, journeyID string) ([]JourneyStep, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, journey_id, step_order, step_type, title, config, delay_hours
		FROM crm_journey_steps WHERE journey_id = $1 ORDER BY step_order
	`, journeyID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []JourneyStep
	for rows.Next() {
		var s JourneyStep
		var cfg []byte
		if err := rows.Scan(&s.ID, &s.JourneyID, &s.StepOrder, &s.StepType, &s.Title, &cfg, &s.DelayHours); err != nil {
			return nil, err
		}
		_ = json.Unmarshal(cfg, &s.Config)
		out = append(out, s)
	}
	return out, rows.Err()
}

func (r *Repository) CreateJourneyStep(ctx context.Context, journeyID string, in map[string]any) (JourneyStep, error) {
	id, err := r.NextID(ctx, "JST", 100)
	if err != nil {
		return JourneyStep{}, err
	}
	order := intNum(in, "step_order")
	if order <= 0 {
		var max int
		_ = r.pool.QueryRow(ctx, `SELECT COALESCE(MAX(step_order), 0) FROM crm_journey_steps WHERE journey_id = $1`, journeyID).Scan(&max)
		order = max + 1
	}
	stepType := str(in, "step_type")
	if stepType == "" {
		stepType = "wait"
	}
	cfg, _ := json.Marshal(in["config"])
	_, err = r.pool.Exec(ctx, `
		INSERT INTO crm_journey_steps (id, journey_id, step_order, step_type, title, config, delay_hours, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6::jsonb,$7,NOW(),NOW())
	`, id, journeyID, order, stepType, str(in, "title"), cfg, intNum(in, "delay_hours"))
	if err != nil {
		return JourneyStep{}, err
	}
	steps, err := r.ListJourneySteps(ctx, journeyID)
	if err != nil {
		return JourneyStep{}, err
	}
	for _, s := range steps {
		if s.ID == id {
			return s, nil
		}
	}
	return JourneyStep{}, pgx.ErrNoRows
}

func (r *Repository) SeedJourneyStepsIfEmpty(ctx context.Context, journeyID string, steps []JourneyStep) error {
	// On a freshly rebuilt DB the parent crm_journeys row may not exist yet:
	// the boot-time backfill (EnsureJourneySteps in main.go) runs before
	// seed.Run inserts the journeys. Seeding steps for a missing parent
	// violates the journey_id FK (23503), so skip until the journey exists —
	// seed.Run calls this again right after creating the journeys.
	var parentExists bool
	if err := r.pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM crm_journeys WHERE id = $1)`, journeyID).Scan(&parentExists); err != nil {
		return err
	}
	if !parentExists {
		return nil
	}

	var n int
	_ = r.pool.QueryRow(ctx, `SELECT COUNT(*)::int FROM crm_journey_steps WHERE journey_id = $1`, journeyID).Scan(&n)
	if n > 0 {
		return nil
	}
	for _, s := range steps {
		cfg, _ := json.Marshal(s.Config)
		_, err := r.pool.Exec(ctx, `
			INSERT INTO crm_journey_steps (id, journey_id, step_order, step_type, title, config, delay_hours, created_at, updated_at)
			VALUES ($1,$2,$3,$4,$5,$6::jsonb,$7,NOW(),NOW())
		`, s.ID, journeyID, s.StepOrder, s.StepType, s.Title, cfg, s.DelayHours)
		if err != nil {
			return err
		}
	}
	return nil
}

func (r *Repository) ListActiveJourneysByTrigger(ctx context.Context, triggerHint string) ([]string, error) {
	hint := strings.ToLower(strings.TrimSpace(triggerHint))
	rows, err := r.pool.Query(ctx, `
		SELECT id FROM crm_journeys WHERE status = 'active' AND LOWER(trigger) LIKE '%' || $1 || '%'
	`, hint)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

type EnrollInput struct {
	ContactID    string
	LeadID       string
	SubjectEmail string
	SubjectName  string
}

func (r *Repository) EnrollInJourney(ctx context.Context, journeyID string, in EnrollInput) (JourneyEnrollment, error) {
	if existing, err := r.GetActiveEnrollment(ctx, journeyID, in.ContactID, in.LeadID); err == nil {
		return existing, nil
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return JourneyEnrollment{}, err
	}
	id, err := r.NextID(ctx, "ENR", 1000)
	if err != nil {
		return JourneyEnrollment{}, err
	}
	now := time.Now().UTC()
	_, err = r.pool.Exec(ctx, `
		INSERT INTO crm_journey_enrollments (
			id, journey_id, contact_id, lead_id, subject_email, subject_name,
			status, current_step, next_run_at, enrolled_at, updated_at
		) VALUES ($1,$2,$3,$4,$5,$6,'active',1,$7,$7,$7)
	`, id, journeyID, nullStr(in.ContactID), nullStr(in.LeadID), in.SubjectEmail, in.SubjectName, now)
	if err != nil {
		return JourneyEnrollment{}, err
	}
	_, _ = r.pool.Exec(ctx, `UPDATE crm_journeys SET enrolled = enrolled + 1, updated_at = NOW() WHERE id = $1`, journeyID)
	return r.GetJourneyEnrollment(ctx, id)
}

func (r *Repository) GetJourneyEnrollment(ctx context.Context, id string) (JourneyEnrollment, error) {
	var e JourneyEnrollment
	var contactID, leadID *string
	err := r.pool.QueryRow(ctx, `
		SELECT id, journey_id, contact_id, lead_id, subject_email, subject_name,
		       status, current_step, next_run_at, enrolled_at
		FROM crm_journey_enrollments WHERE id = $1
	`, id).Scan(&e.ID, &e.JourneyID, &contactID, &leadID, &e.SubjectEmail, &e.SubjectName,
		&e.Status, &e.CurrentStep, &e.NextRunAt, &e.EnrolledAt)
	if contactID != nil {
		e.ContactID = *contactID
	}
	if leadID != nil {
		e.LeadID = *leadID
	}
	return e, err
}

func (r *Repository) ListJourneyEnrollments(ctx context.Context, journeyID string, limit int) ([]JourneyEnrollment, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := r.pool.Query(ctx, `
		SELECT id, journey_id, contact_id, lead_id, subject_email, subject_name,
		       status, current_step, next_run_at, enrolled_at
		FROM crm_journey_enrollments
		WHERE ($1 = '' OR journey_id = $1)
		ORDER BY enrolled_at DESC LIMIT $2
	`, journeyID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []JourneyEnrollment
	for rows.Next() {
		var e JourneyEnrollment
		var contactID, leadID *string
		if err := rows.Scan(&e.ID, &e.JourneyID, &contactID, &leadID, &e.SubjectEmail, &e.SubjectName,
			&e.Status, &e.CurrentStep, &e.NextRunAt, &e.EnrolledAt); err != nil {
			return nil, err
		}
		if contactID != nil {
			e.ContactID = *contactID
		}
		if leadID != nil {
			e.LeadID = *leadID
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

func (r *Repository) GetActiveEnrollment(ctx context.Context, journeyID, contactID, leadID string) (JourneyEnrollment, error) {
	var e JourneyEnrollment
	var cid, lid *string
	err := r.pool.QueryRow(ctx, `
		SELECT id, journey_id, contact_id, lead_id, subject_email, subject_name,
		       status, current_step, next_run_at, enrolled_at
		FROM crm_journey_enrollments
		WHERE journey_id = $1 AND status = 'active'
		  AND (
		    ($2 <> '' AND contact_id = $2)
		    OR ($3 <> '' AND lead_id = $3)
		  )
		ORDER BY enrolled_at DESC
		LIMIT 1
	`, journeyID, contactID, leadID).Scan(
		&e.ID, &e.JourneyID, &cid, &lid, &e.SubjectEmail, &e.SubjectName,
		&e.Status, &e.CurrentStep, &e.NextRunAt, &e.EnrolledAt,
	)
	if cid != nil {
		e.ContactID = *cid
	}
	if lid != nil {
		e.LeadID = *lid
	}
	return e, err
}

func (r *Repository) ClaimDueEnrollments(ctx context.Context, limit int) ([]JourneyEnrollment, error) {
	if limit <= 0 {
		limit = 32
	}
	rows, err := r.pool.Query(ctx, `
		UPDATE crm_journey_enrollments e
		SET next_run_at = NOW() + INTERVAL '5 minutes', updated_at = NOW()
		FROM (
			SELECT id FROM crm_journey_enrollments
			WHERE status = 'active' AND next_run_at <= NOW()
			ORDER BY next_run_at
			LIMIT $1
			FOR UPDATE SKIP LOCKED
		) due
		WHERE e.id = due.id
		RETURNING e.id, e.journey_id, e.contact_id, e.lead_id, e.subject_email, e.subject_name,
		          e.status, e.current_step, e.next_run_at, e.enrolled_at
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []JourneyEnrollment
	for rows.Next() {
		var e JourneyEnrollment
		var contactID, leadID *string
		if err := rows.Scan(&e.ID, &e.JourneyID, &contactID, &leadID, &e.SubjectEmail, &e.SubjectName,
			&e.Status, &e.CurrentStep, &e.NextRunAt, &e.EnrolledAt); err != nil {
			return nil, err
		}
		if contactID != nil {
			e.ContactID = *contactID
		}
		if leadID != nil {
			e.LeadID = *leadID
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

func (r *Repository) GetJourneyStepAtOrder(ctx context.Context, journeyID string, order int) (JourneyStep, error) {
	var s JourneyStep
	var cfg []byte
	err := r.pool.QueryRow(ctx, `
		SELECT id, journey_id, step_order, step_type, title, config, delay_hours
		FROM crm_journey_steps WHERE journey_id = $1 AND step_order = $2
	`, journeyID, order).Scan(&s.ID, &s.JourneyID, &s.StepOrder, &s.StepType, &s.Title, &cfg, &s.DelayHours)
	if err != nil {
		return s, err
	}
	_ = json.Unmarshal(cfg, &s.Config)
	return s, nil
}

func (r *Repository) AdvanceEnrollment(ctx context.Context, enrollmentID string, nextStep int, delayHours int, completed bool) error {
	if completed {
		_, err := r.pool.Exec(ctx, `
			UPDATE crm_journey_enrollments
			SET status = 'completed', completed_at = NOW(), updated_at = NOW()
			WHERE id = $1
		`, enrollmentID)
		return err
	}
	nextRun := time.Now().UTC().Add(time.Duration(delayHours) * time.Hour)
	_, err := r.pool.Exec(ctx, `
		UPDATE crm_journey_enrollments
		SET current_step = $2, next_run_at = $3, updated_at = NOW()
		WHERE id = $1
	`, enrollmentID, nextStep, nextRun)
	return err
}

func (r *Repository) LogJourneyStep(ctx context.Context, enrollmentID, stepID string, order int, stepType, status, detail string) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO crm_journey_step_logs (enrollment_id, step_id, step_order, step_type, status, detail)
		VALUES ($1,$2,$3,$4,$5,$6)
	`, enrollmentID, nullStr(stepID), order, stepType, status, detail)
	return err
}

func (r *Repository) BumpJourneyConversion(ctx context.Context, journeyID string) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE crm_journeys SET
			conversion = CASE WHEN enrolled > 0
				THEN LEAST(100, conversion + (100.0 / GREATEST(enrolled, 1)))
				ELSE conversion END,
			updated_at = NOW()
		WHERE id = $1
	`, journeyID)
	return err
}

func (r *Repository) ActivateJourney(ctx context.Context, journeyID string) error {
	tag, err := r.pool.Exec(ctx, `UPDATE crm_journeys SET status = 'active', updated_at = NOW() WHERE id = $1`, journeyID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}
