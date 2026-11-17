package store

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

// SalesTarget is an organisation or owner quota for a calendar period.
type SalesTarget struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Period    string    `json:"period"`
	TargetUSD float64   `json:"target_usd"`
	Owner     string    `json:"owner"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type SalesTargetInput struct {
	Name      string  `json:"name"`
	Period    string  `json:"period"`
	TargetUSD float64 `json:"target_usd"`
	Owner     string  `json:"owner"`
	Status    string  `json:"status"`
}

func currentSalesPeriod(now time.Time) string {
	q := (int(now.Month())-1)/3 + 1
	return fmt.Sprintf("%d-Q%d", now.Year(), q)
}

func periodBounds(period string, now time.Time) (time.Time, time.Time) {
	period = strings.TrimSpace(period)
	if period == "" {
		period = currentSalesPeriod(now)
	}
	// YYYY-Qn
	var year, q int
	if _, err := fmt.Sscanf(period, "%d-Q%d", &year, &q); err == nil && q >= 1 && q <= 4 {
		startMonth := time.Month((q-1)*3 + 1)
		start := time.Date(year, startMonth, 1, 0, 0, 0, 0, time.UTC)
		end := start.AddDate(0, 3, 0)
		return start, end
	}
	// YYYY-MM
	var month int
	if _, err := fmt.Sscanf(period, "%d-%d", &year, &month); err == nil && month >= 1 && month <= 12 {
		start := time.Date(year, time.Month(month), 1, 0, 0, 0, 0, time.UTC)
		return start, start.AddDate(0, 1, 0)
	}
	// Fallback: current quarter
	p := currentSalesPeriod(now)
	return periodBounds(p, now)
}

func (r *Repository) ListSalesTargets(ctx context.Context, period string) ([]SalesTarget, error) {
	rows, err := r.db(ctx).Query(ctx, `
		SELECT id, name, period, target_usd, owner, status, created_at, updated_at
		FROM crm_sales_targets
		WHERE ($1 = '' OR period = $1)
		ORDER BY period DESC, name`, period)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []SalesTarget
	for rows.Next() {
		var t SalesTarget
		if err := rows.Scan(&t.ID, &t.Name, &t.Period, &t.TargetUSD, &t.Owner, &t.Status, &t.CreatedAt, &t.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func (r *Repository) CreateSalesTarget(ctx context.Context, in SalesTargetInput) (SalesTarget, error) {
	name := strings.TrimSpace(in.Name)
	if name == "" {
		return SalesTarget{}, fmt.Errorf("%w: name is required", ErrInvalidInput)
	}
	period := strings.TrimSpace(in.Period)
	if period == "" {
		period = currentSalesPeriod(time.Now().UTC())
	}
	status := strings.ToLower(strings.TrimSpace(in.Status))
	if status == "" {
		status = "active"
	}
	id := r.NewID()
	now := time.Now().UTC()
	_, err := r.db(ctx).Exec(ctx, `
		INSERT INTO crm_sales_targets (id, name, period, target_usd, owner, status, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$7)`,
		id, name, period, in.TargetUSD, strings.TrimSpace(in.Owner), status, now)
	if err != nil {
		return SalesTarget{}, err
	}
	return r.GetSalesTarget(ctx, id)
}

func (r *Repository) GetSalesTarget(ctx context.Context, id string) (SalesTarget, error) {
	var t SalesTarget
	err := r.db(ctx).QueryRow(ctx, `
		SELECT id, name, period, target_usd, owner, status, created_at, updated_at
		FROM crm_sales_targets WHERE id = $1`, id).
		Scan(&t.ID, &t.Name, &t.Period, &t.TargetUSD, &t.Owner, &t.Status, &t.CreatedAt, &t.UpdatedAt)
	if err != nil {
		return t, err
	}
	return t, nil
}

func (r *Repository) PatchSalesTarget(ctx context.Context, id string, patch map[string]any) (SalesTarget, error) {
	cur, err := r.GetSalesTarget(ctx, id)
	if err != nil {
		return cur, err
	}
	if v, ok := patch["name"].(string); ok && strings.TrimSpace(v) != "" {
		cur.Name = strings.TrimSpace(v)
	}
	if v, ok := patch["period"].(string); ok && strings.TrimSpace(v) != "" {
		cur.Period = strings.TrimSpace(v)
	}
	if v, ok := patch["owner"].(string); ok {
		cur.Owner = strings.TrimSpace(v)
	}
	if v, ok := patch["status"].(string); ok && strings.TrimSpace(v) != "" {
		cur.Status = strings.ToLower(strings.TrimSpace(v))
	}
	if v, ok := patch["target_usd"].(float64); ok {
		cur.TargetUSD = v
	}
	_, err = r.db(ctx).Exec(ctx, `
		UPDATE crm_sales_targets
		SET name=$2, period=$3, target_usd=$4, owner=$5, status=$6, updated_at=NOW()
		WHERE id=$1`,
		id, cur.Name, cur.Period, cur.TargetUSD, cur.Owner, cur.Status)
	if err != nil {
		return cur, err
	}
	return r.GetSalesTarget(ctx, id)
}

func (r *Repository) DeleteSalesTarget(ctx context.Context, id string) error {
	tag, err := r.db(ctx).Exec(ctx, `DELETE FROM crm_sales_targets WHERE id=$1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

func (r *Repository) wonDealAmount(ctx context.Context, start, end time.Time, owner string) (float64, error) {
	var total float64
	err := r.db(ctx).QueryRow(ctx, `
		SELECT COALESCE(SUM(amount), 0)::float8
		FROM crm_deals
		WHERE stage = 'won'
		  AND COALESCE(won_at, updated_at) >= $1
		  AND COALESCE(won_at, updated_at) < $2
		  AND ($3 = '' OR lower(owner) = lower($3))`,
		start, end, owner).Scan(&total)
	return total, err
}

// SalesTargetProgress rolls won deal amounts against active quotas for a period.
// When no crm_sales_targets rows exist, falls back to marketing budget won_target.
func (r *Repository) SalesTargetProgress(ctx context.Context, period string) (map[string]any, error) {
	now := time.Now().UTC()
	if strings.TrimSpace(period) == "" {
		period = currentSalesPeriod(now)
	}
	start, end := periodBounds(period, now)

	targets, err := r.ListSalesTargets(ctx, period)
	if err != nil {
		return nil, err
	}
	active := make([]SalesTarget, 0, len(targets))
	for _, t := range targets {
		if strings.EqualFold(t.Status, "active") && t.TargetUSD > 0 {
			active = append(active, t)
		}
	}

	source := "sales_targets"
	if len(active) == 0 {
		// Fallback: marketing budget plans for this quarter.
		rows, qerr := r.db(ctx).Query(ctx, `
			SELECT id, name, quarter, owner, won_target
			FROM crm_budget_plans
			WHERE won_target > 0 AND ($1 = '' OR quarter = $1 OR quarter = $2)
			ORDER BY name`, period, strings.TrimPrefix(period, fmt.Sprintf("%d-", now.Year())))
		if qerr == nil {
			defer rows.Close()
			for rows.Next() {
				var id, name, quarter, owner string
				var won float64
				if rows.Scan(&id, &name, &quarter, &owner, &won) == nil {
					active = append(active, SalesTarget{
						ID: id, Name: name, Period: quarter, TargetUSD: won, Owner: owner, Status: "active",
					})
				}
			}
			if len(active) > 0 {
				source = "budget_plans"
			}
		}
	}

	orgMade, err := r.wonDealAmount(ctx, start, end, "")
	if err != nil {
		return nil, err
	}

	var targetTotal float64
	var hit int
	details := make([]map[string]any, 0, len(active))
	for _, t := range active {
		made := orgMade
		if strings.TrimSpace(t.Owner) != "" {
			made, err = r.wonDealAmount(ctx, start, end, t.Owner)
			if err != nil {
				return nil, err
			}
		}
		targetTotal += t.TargetUSD
		attained := t.TargetUSD > 0 && made >= t.TargetUSD
		if attained {
			hit++
		}
		pct := 0.0
		if t.TargetUSD > 0 {
			pct = made / t.TargetUSD * 100
		}
		details = append(details, map[string]any{
			"id": t.ID, "name": t.Name, "owner": t.Owner,
			"target_usd": t.TargetUSD, "made_usd": made,
			"attainment_pct": pct, "hit": attained,
		})
	}

	attainment := 0.0
	if targetTotal > 0 {
		attainment = orgMade / targetTotal * 100
		if attainment > 999 {
			attainment = 999
		}
	}

	return map[string]any{
		"period":         period,
		"period_start":   start.Format("2006-01-02"),
		"period_end":     end.Format("2006-01-02"),
		"source":         source,
		"target_usd":     targetTotal,
		"made_usd":       orgMade,
		"attainment_pct": attainment,
		"targets_total":  len(active),
		"targets_hit":    hit,
		"targets":        details,
	}, nil
}
