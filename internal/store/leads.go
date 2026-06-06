package store

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/iag/crm/backend/internal/models"
)

func scanLead(row pgx.Row) (models.Lead, error) {
	var l models.Lead
	err := row.Scan(
		&l.ID, &l.Name, &l.Company, &l.Email, &l.Phone, &l.Source, &l.Segment,
		&l.Region, &l.Score, &l.Status, &l.Owner, &l.Notes, &l.CreatedAt, &l.UpdatedAt,
	)
	return l, err
}

func (r *Repository) ListLeads(ctx context.Context, opts ListOpts) ([]models.Lead, int, error) {
	opts.Limit = clampLimit(opts.Limit)
	where := []string{"1=1"}
	args := []any{}
	i := 1
	if opts.Owner != "" {
		where = append(where, fmt.Sprintf("owner = $%d", i))
		args = append(args, opts.Owner)
		i++
	}
	if opts.Status != "" {
		where = append(where, fmt.Sprintf("status = $%d", i))
		args = append(args, opts.Status)
		i++
	}
	whereSQL := strings.Join(where, " AND ")
	var total int
	if err := r.pool.QueryRow(ctx, "SELECT COUNT(*)::int FROM crm_leads WHERE "+whereSQL, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	args = append(args, opts.Limit, opts.Offset)
	rows, err := r.pool.Query(ctx, `
		SELECT id, name, company, email, phone, source, segment, region, score, status, owner, notes, created_at, updated_at
		FROM crm_leads WHERE `+whereSQL+` ORDER BY score DESC, created_at DESC LIMIT $`+fmt.Sprint(i)+` OFFSET $`+fmt.Sprint(i+1), args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var out []models.Lead
	for rows.Next() {
		l, err := scanLead(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, l)
	}
	return out, total, rows.Err()
}

func (r *Repository) GetLead(ctx context.Context, id string) (models.Lead, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT id, name, company, email, phone, source, segment, region, score, status, owner, notes, created_at, updated_at
		FROM crm_leads WHERE id = $1
	`, id)
	return scanLead(row)
}

type LeadInput struct {
	Name    string `json:"name"`
	Company string `json:"company"`
	Email   string `json:"email"`
	Phone   string `json:"phone"`
	Source  string `json:"source"`
	Segment string `json:"segment"`
	Region  string `json:"region"`
	Owner   string `json:"owner"`
	Notes   string `json:"notes"`
	Score   int    `json:"score"`
	Status  string `json:"status"`
}

func (r *Repository) CreateLead(ctx context.Context, in LeadInput) (models.Lead, error) {
	id, err := r.NextID(ctx, "LEAD", 200)
	if err != nil {
		return models.Lead{}, err
	}
	if in.Status == "" {
		in.Status = "qualifying"
	}
	if in.Score == 0 {
		in.Score = 65
	}
	now := time.Now().UTC()
	_, err = r.pool.Exec(ctx, `
		INSERT INTO crm_leads (id, name, company, email, phone, source, segment, region, score, status, owner, notes, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$13)
	`, id, in.Name, in.Company, in.Email, in.Phone, in.Source, in.Segment, in.Region, in.Score, in.Status, in.Owner, in.Notes, now)
	if err != nil {
		return models.Lead{}, err
	}
	return r.GetLead(ctx, id)
}

func (r *Repository) PatchLead(ctx context.Context, id string, patch map[string]any) (models.Lead, error) {
	sets := []string{"updated_at = NOW()"}
	args := []any{}
	i := 1
	add := func(col string, val any) {
		sets = append(sets, fmt.Sprintf("%s = $%d", col, i))
		args = append(args, val)
		i++
	}
	for k, col := range map[string]string{
		"name": "name", "company": "company", "email": "email", "phone": "phone",
		"source": "source", "segment": "segment", "region": "region", "status": "status", "owner": "owner", "notes": "notes",
	} {
		if v, ok := patch[k].(string); ok {
			add(col, v)
		}
	}
	if v, ok := patch["score"].(float64); ok {
		add("score", int(v))
	}
	if len(sets) == 1 {
		return r.GetLead(ctx, id)
	}
	args = append(args, id)
	_, err := r.pool.Exec(ctx, `UPDATE crm_leads SET `+strings.Join(sets, ", ")+` WHERE id = $`+fmt.Sprint(i), args...)
	if err != nil {
		return models.Lead{}, err
	}
	return r.GetLead(ctx, id)
}

func (r *Repository) BumpLeadScore(ctx context.Context, leadID string, delta int) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE crm_leads SET score = LEAST(100, score + $2), updated_at = NOW() WHERE id = $1
	`, leadID, delta)
	return err
}

func (r *Repository) ConvertLead(ctx context.Context, leadID string, dealIn DealInput) (models.Deal, error) {
	lead, err := r.GetLead(ctx, leadID)
	if err != nil {
		return models.Deal{}, err
	}
	if dealIn.Name == "" {
		dealIn.Name = lead.Company
		if dealIn.Name == "" {
			dealIn.Name = lead.Name
		}
	}
	if dealIn.Owner == "" {
		dealIn.Owner = lead.Owner
	}
	deal, err := r.CreateDeal(ctx, dealIn)
	if err != nil {
		return models.Deal{}, err
	}
	_, _ = r.pool.Exec(ctx, `UPDATE crm_leads SET status = 'converted', converted_deal_id = $2, updated_at = NOW() WHERE id = $1`, leadID, deal.ID)
	return deal, nil
}
