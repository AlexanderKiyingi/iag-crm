package store

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/iag/crm/backend/internal/models"
)

// leadColumns is the single source of truth for the lead read shape. It was
// repeated verbatim in three queries; 0012 adding five columns to two of them
// and not the third is exactly the drift this prevents.
const leadColumns = `id, name, company, email, phone, source, segment, region, score, status, owner, notes,
	       account_id, account_name, next_follow_up_at, estimated_value, currency, attrs, created_at, updated_at`

func scanLead(row pgx.Row) (models.Lead, error) {
	var l models.Lead
	var attrs []byte
	// account_id is nullable (a lead whose customer matches no account) and
	// estimated_value / next_follow_up_at are unset until someone fills them in.
	// Scanning any of them into a plain value fails the WHOLE query, not just
	// the row — the defect that took GET /tickets, /campaigns and /accounts down
	// on this service already.
	var accountID *string
	var nextFollowUp *time.Time
	var estimatedValue *float64
	err := row.Scan(
		&l.ID, &l.Name, &l.Company, &l.Email, &l.Phone, &l.Source, &l.Segment,
		&l.Region, &l.Score, &l.Status, &l.Owner, &l.Notes,
		&accountID, &l.Account, &nextFollowUp, &estimatedValue, &l.Currency,
		&attrs, &l.CreatedAt, &l.UpdatedAt,
	)
	if err != nil {
		return l, err
	}
	if accountID != nil {
		l.AccountID = *accountID
	}
	l.NextFollowUpAt = nextFollowUp
	l.EstimatedValue = estimatedValue
	l.Attrs = decodeAttrs(attrs)
	return l, nil
}

func (r *Repository) ListLeads(ctx context.Context, opts ListOpts) ([]models.Lead, int, error) {
	opts.Limit = clampLimit(opts.Limit)
	where := []string{"1=1"}
	args := []any{}
	i := 1
	where, args = applyScope(opts, "owner", where, args, &i)
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
	if err := r.db(ctx).QueryRow(ctx, "SELECT COUNT(*)::int FROM crm_leads WHERE "+whereSQL, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	args = append(args, opts.Limit, opts.Offset)
	rows, err := r.db(ctx).Query(ctx, `
		SELECT `+leadColumns+`
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
	row := r.db(ctx).QueryRow(ctx, `
		SELECT `+leadColumns+`
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
	// Account is the free-text customer name; AccountID wins when both are sent.
	// A name matching no account is not an error — the lead is simply unlinked,
	// which is what CreateDeal and CreateContact already do.
	Account   string `json:"account"`
	AccountID string `json:"account_id"`
	// Promoted out of attrs by 0012. Pointers so "not supplied" stays distinct
	// from "explicitly zero" — an estimated value of 0 is a real answer.
	NextFollowUpAt *time.Time `json:"next_follow_up_at"`
	EstimatedValue *float64   `json:"estimated_value"`
	Currency       string     `json:"currency"`
	// Attrs carries client fields with no promoted column. See
	// db/migrations/0008_entity_attrs.sql.
	Attrs map[string]any `json:"attrs"`
}

func (r *Repository) CreateLead(ctx context.Context, in LeadInput) (models.Lead, error) {
	id := r.NewID()
	if in.Status == "" {
		in.Status = "qualifying"
	}
	// No default score. 65 was invented for a lead nobody had scored, and it is
	// not inert: ListLeads orders on `score DESC, created_at DESC`, so every
	// unscored lead outranked one a human had honestly scored below 65, and the
	// list read as a ranking when nothing had been ranked. A caller that scores
	// its leads still sends one; the rest read 0, which is true, and the ordering
	// falls through to created_at DESC.
	accountID := in.AccountID
	if accountID == "" && in.Account != "" {
		resolved, err := r.resolveAccountID(ctx, in.Account)
		if err != nil {
			return models.Lead{}, err
		}
		accountID = resolved
	}
	now := time.Now().UTC()
	_, err := r.db(ctx).Exec(ctx, `
		INSERT INTO crm_leads (id, name, company, email, phone, source, segment, region, score, status, owner, notes,
			account_id, account_name, next_follow_up_at, estimated_value, currency, attrs, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$19)
	`, id, in.Name, in.Company, in.Email, in.Phone, in.Source, in.Segment, in.Region, in.Score, in.Status, in.Owner, in.Notes,
		nullStr(accountID), in.Account, in.NextFollowUpAt, in.EstimatedValue, in.Currency, encodeAttrs(in.Attrs), now)
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
		"account": "account_name", "currency": "currency",
	} {
		if v, ok := patch[k].(string); ok {
			add(col, v)
		}
	}
	if v, ok := patch["score"].(float64); ok {
		add("score", int(v))
	}
	if v, ok := patch["next_follow_up_at"]; ok {
		add("next_follow_up_at", parsePatchTime(v))
	}
	if v, ok := patch["estimated_value"]; ok {
		add("estimated_value", parsePatchNumber(v))
	}
	if attrs, ok := patchAttrs(patch); ok {
		add("attrs", encodeAttrs(attrs))
	}
	// Re-point the foreign key when the customer name changes. Writing
	// account_name alone leaves account_id on the previous account, so the screen
	// shows one company and every join — /accounts/:id/360, the rollups — still
	// reports the other. That exact split was fixed on deals and contacts; leads
	// gain the link and the fix together.
	if v, ok := patch["account"].(string); ok {
		accountID, err := r.resolveAccountID(ctx, v)
		if err != nil {
			return models.Lead{}, err
		}
		add("account_id", nullStr(accountID))
	}
	if len(sets) == 1 {
		return r.GetLead(ctx, id)
	}
	args = append(args, id)
	_, err := r.db(ctx).Exec(ctx, `UPDATE crm_leads SET `+strings.Join(sets, ", ")+` WHERE id = $`+fmt.Sprint(i), args...)
	if err != nil {
		return models.Lead{}, err
	}
	return r.GetLead(ctx, id)
}

func (r *Repository) BumpLeadScore(ctx context.Context, leadID string, delta int) error {
	_, err := r.db(ctx).Exec(ctx, `
		UPDATE crm_leads SET score = LEAST(100, score + $2), updated_at = NOW() WHERE id = $1
	`, leadID, delta)
	return err
}

func (r *Repository) ConvertLead(ctx context.Context, leadID string, dealIn DealInput) (models.Deal, error) {
	var deal models.Deal
	err := r.WithinTx(ctx, func(ctx context.Context) error {
		lead, err := r.GetLead(ctx, leadID)
		if err != nil {
			return err
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
		deal, err = r.CreateDeal(ctx, dealIn)
		if err != nil {
			return err
		}
		// Mark the lead converted in the same transaction as the deal insert.
		// If this fails the whole conversion rolls back, so a retry can't leave
		// an orphaned deal behind or double-convert the lead.
		ct, err := r.db(ctx).Exec(ctx, `UPDATE crm_leads SET status = 'converted', converted_deal_id = $2, updated_at = NOW() WHERE id = $1`, leadID, deal.ID)
		if err != nil {
			return err
		}
		if ct.RowsAffected() == 0 {
			return pgx.ErrNoRows
		}
		return nil
	})
	if err != nil {
		return models.Deal{}, err
	}
	return deal, nil
}
