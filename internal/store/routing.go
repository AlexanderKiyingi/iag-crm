package store

import (
	"context"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/iag/crm/backend/internal/models"
)

type Territory struct {
	ID         string    `json:"id"`
	Name       string    `json:"name"`
	Region     string    `json:"region"`
	OwnerEmail string    `json:"owner_email"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

type AssignmentRule struct {
	ID         string    `json:"id"`
	Name       string    `json:"name"`
	Region     string    `json:"region"`
	Segment    string    `json:"segment"`
	MinScore   int       `json:"min_score"`
	OwnerEmail string    `json:"owner_email"`
	Priority   int       `json:"priority"`
	Active     bool      `json:"active"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

func (r *Repository) ListTerritories(ctx context.Context) ([]Territory, error) {
	rows, err := r.db(ctx).Query(ctx, `
		SELECT id, name, region, owner_email, created_at, updated_at
		FROM crm_territories ORDER BY name ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Territory
	for rows.Next() {
		var t Territory
		if err := rows.Scan(&t.ID, &t.Name, &t.Region, &t.OwnerEmail, &t.CreatedAt, &t.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func (r *Repository) CreateTerritory(ctx context.Context, name, region, ownerEmail string) (Territory, error) {
	id := r.NewID()
	now := time.Now().UTC()
	_, err := r.db(ctx).Exec(ctx, `
		INSERT INTO crm_territories (id, name, region, owner_email, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$5)
	`, id, name, region, ownerEmail, now)
	if err != nil {
		return Territory{}, err
	}
	return Territory{ID: id, Name: name, Region: region, OwnerEmail: ownerEmail, CreatedAt: now, UpdatedAt: now}, nil
}

func (r *Repository) ListAssignmentRules(ctx context.Context) ([]AssignmentRule, error) {
	rows, err := r.db(ctx).Query(ctx, `
		SELECT id, name, region, segment, min_score, owner_email, priority, active, created_at, updated_at
		FROM crm_assignment_rules ORDER BY priority ASC, name ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []AssignmentRule
	for rows.Next() {
		var rule AssignmentRule
		if err := rows.Scan(
			&rule.ID, &rule.Name, &rule.Region, &rule.Segment, &rule.MinScore,
			&rule.OwnerEmail, &rule.Priority, &rule.Active, &rule.CreatedAt, &rule.UpdatedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, rule)
	}
	return out, rows.Err()
}

func (r *Repository) CreateAssignmentRule(ctx context.Context, rule AssignmentRule) (AssignmentRule, error) {
	id := r.NewID()
	now := time.Now().UTC()
	if rule.Priority == 0 {
		rule.Priority = 100
	}
	_, err := r.db(ctx).Exec(ctx, `
		INSERT INTO crm_assignment_rules
			(id, name, region, segment, min_score, owner_email, priority, active, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$9)
	`, id, rule.Name, rule.Region, rule.Segment, rule.MinScore, rule.OwnerEmail, rule.Priority, rule.Active, now)
	if err != nil {
		return AssignmentRule{}, err
	}
	rule.ID = id
	rule.CreatedAt = now
	rule.UpdatedAt = now
	return rule, nil
}

// MatchAssignmentOwner returns the owner email for a lead based on active rules
// (first match by priority), then territories by region.
func (r *Repository) MatchAssignmentOwner(ctx context.Context, region, segment string, score int) (string, error) {
	rules, err := r.ListAssignmentRules(ctx)
	if err != nil {
		return "", err
	}
	region = strings.TrimSpace(strings.ToLower(region))
	segment = strings.TrimSpace(strings.ToLower(segment))
	for _, rule := range rules {
		if !rule.Active || rule.OwnerEmail == "" {
			continue
		}
		if rule.MinScore > score {
			continue
		}
		if rr := strings.TrimSpace(strings.ToLower(rule.Region)); rr != "" && rr != region {
			continue
		}
		if ss := strings.TrimSpace(strings.ToLower(rule.Segment)); ss != "" && ss != segment {
			continue
		}
		return rule.OwnerEmail, nil
	}
	if region == "" {
		return "", nil
	}
	var owner string
	err = r.db(ctx).QueryRow(ctx, `
		SELECT owner_email FROM crm_territories
		WHERE LOWER(region) = LOWER($1) AND owner_email <> ''
		ORDER BY name ASC LIMIT 1
	`, region).Scan(&owner)
	if err == pgx.ErrNoRows {
		return "", nil
	}
	return owner, err
}

// SetLeadScore updates score and returns the lead.
func (r *Repository) SetLeadScore(ctx context.Context, id string, score int) (models.Lead, error) {
	if score < 0 {
		score = 0
	}
	if score > 100 {
		score = 100
	}
	tag, err := r.db(ctx).Exec(ctx, `
		UPDATE crm_leads SET score = $2, updated_at = NOW() WHERE id = $1
	`, id, score)
	if err != nil {
		return models.Lead{}, err
	}
	if tag.RowsAffected() == 0 {
		return models.Lead{}, pgx.ErrNoRows
	}
	return r.GetLead(ctx, id)
}
