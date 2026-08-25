package store

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/iag/crm/backend/internal/models"
)

func scanQuote(row pgx.Row) (models.Quote, error) {
	var q models.Quote
	var accountID, dealID *string
	var validUntil *time.Time
	var lineRaw []byte
	var financeAR, contractRef *string
	err := row.Scan(
		&q.ID, &q.Ref, &accountID, &q.Account, &dealID, &q.Template, &q.Currency,
		&q.Incoterms, &q.PaymentTerms, &validUntil, &q.Total, &q.Status, &q.Version,
		&q.Owner, &lineRaw, &financeAR, &contractRef, &q.CreatedAt, &q.UpdatedAt,
	)
	if financeAR != nil {
		q.FinanceARRef = *financeAR
	}
	if contractRef != nil {
		q.ContractRef = *contractRef
	}
	if err != nil {
		return q, err
	}
	if accountID != nil {
		q.AccountID = *accountID
	}
	if dealID != nil {
		q.DealID = *dealID
	}
	q.ValidUntil = validUntil
	if len(lineRaw) > 0 {
		_ = json.Unmarshal(lineRaw, &q.LineItems)
	}
	return q, nil
}

func (r *Repository) ListQuotes(ctx context.Context, opts ListOpts) ([]models.Quote, int, error) {
	opts.Limit = clampLimit(opts.Limit)
	var total int
	if err := r.db(ctx).QueryRow(ctx, `SELECT COUNT(*)::int FROM crm_quotes`).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := r.db(ctx).Query(ctx, `
		SELECT id, ref, account_id, account_name, deal_id, template, currency, incoterms, payment_terms,
		       valid_until, total, status, version, owner, line_items, finance_ar_ref, contract_ref, created_at, updated_at
		FROM crm_quotes ORDER BY created_at DESC LIMIT $1 OFFSET $2
	`, opts.Limit, opts.Offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var out []models.Quote
	for rows.Next() {
		q, err := scanQuote(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, q)
	}
	return out, total, rows.Err()
}

type QuoteInput struct {
	Ref          string                 `json:"ref"`
	Account      string                 `json:"account"`
	AccountID    string                 `json:"account_id"`
	DealID       string                 `json:"deal_id"`
	Template     string                 `json:"template"`
	Currency     string                 `json:"currency"`
	Incoterms    string                 `json:"incoterms"`
	PaymentTerms string                 `json:"payment_terms"`
	ValidUntil   *time.Time             `json:"valid_until"`
	Total        float64                `json:"total"`
	Owner        string                 `json:"owner"`
	LineItems    []models.QuoteLineItem `json:"line_items"`
}

func (r *Repository) CreateQuote(ctx context.Context, in QuoteInput) (models.Quote, error) {
	id, err := r.NextID(ctx, "QTE", 500)
	if err != nil {
		return models.Quote{}, err
	}
	if in.Ref == "" {
		in.Ref = id
	}
	if in.Currency == "" {
		in.Currency = "USD"
	}
	accountID := in.AccountID
	if accountID == "" && in.Account != "" {
		resolved, err := r.resolveAccountID(ctx, in.Account)
		if err != nil {
			return models.Quote{}, err
		}
		accountID = resolved
	}
	lineRaw, _ := json.Marshal(in.LineItems)
	now := time.Now().UTC()
	_, err = r.db(ctx).Exec(ctx, `
		INSERT INTO crm_quotes (id, ref, account_id, account_name, deal_id, template, currency, incoterms, payment_terms,
			valid_until, total, status, version, owner, line_items, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,'draft',1,$12,$13,$14,$14)
	`, id, in.Ref, nullStr(accountID), in.Account, nullStr(in.DealID), in.Template, in.Currency,
		in.Incoterms, in.PaymentTerms, in.ValidUntil, in.Total, in.Owner, lineRaw, now)
	if err != nil {
		return models.Quote{}, err
	}
	row := r.db(ctx).QueryRow(ctx, `
		SELECT id, ref, account_id, account_name, deal_id, template, currency, incoterms, payment_terms,
		       valid_until, total, status, version, owner, line_items, finance_ar_ref, contract_ref, created_at, updated_at
		FROM crm_quotes WHERE id = $1
	`, id)
	return scanQuote(row)
}

func (r *Repository) GetQuote(ctx context.Context, id string) (models.Quote, error) {
	row := r.db(ctx).QueryRow(ctx, `
		SELECT id, ref, account_id, account_name, deal_id, template, currency, incoterms, payment_terms,
		       valid_until, total, status, version, owner, line_items, finance_ar_ref, contract_ref, created_at, updated_at
		FROM crm_quotes WHERE id = $1
	`, id)
	return scanQuote(row)
}

func (r *Repository) PatchQuote(ctx context.Context, id string, patch map[string]any) (models.Quote, error) {
	sets := []string{}
	args := []any{id}
	i := 2
	for _, field := range []struct{ key, col string }{
		{"status", "status"}, {"total", "total"}, {"currency", "currency"},
		{"incoterms", "incoterms"}, {"payment_terms", "payment_terms"}, {"owner", "owner"},
	} {
		if v, ok := patch[field.key]; ok {
			sets = append(sets, fmt.Sprintf("%s = $%d", field.col, i))
			args = append(args, v)
			i++
		}
	}
	if len(sets) == 0 {
		return r.GetQuote(ctx, id)
	}
	sets = append(sets, "updated_at = NOW()", "version = version + 1")
	q := fmt.Sprintf("UPDATE crm_quotes SET %s WHERE id = $1", strings.Join(sets, ", "))
	tag, err := r.db(ctx).Exec(ctx, q, args...)
	if err != nil {
		return models.Quote{}, err
	}
	if tag.RowsAffected() == 0 {
		return models.Quote{}, pgx.ErrNoRows
	}
	return r.GetQuote(ctx, id)
}

func scanActivity(row pgx.Row) (models.Activity, error) {
	var a models.Activity
	var accountID, contactID, dealID, outletRef *string
	err := row.Scan(
		&a.ID, &a.Type, &a.Subject, &a.Body, &accountID, &a.Account, &contactID, &dealID,
		&outletRef, &a.Owner, &a.OccurredAt, &a.CreatedAt,
	)
	if err != nil {
		return a, err
	}
	if accountID != nil {
		a.AccountID = *accountID
	}
	if contactID != nil {
		a.ContactID = *contactID
	}
	if dealID != nil {
		a.DealID = *dealID
	}
	// outlet_ref is a nullable column; scanning NULL into a plain string errors,
	// which surfaced as a 500 on GET /activities whenever any row had no outlet.
	if outletRef != nil {
		a.OutletRef = *outletRef
	}
	return a, nil
}

func (r *Repository) ListActivities(ctx context.Context, opts ListOpts) ([]models.Activity, int, error) {
	opts.Limit = clampLimit(opts.Limit)
	// Optional type filter (comma-separated), so callers can page a single
	// activity kind (task/meeting/call) without over-fetching and filtering client-side.
	types := splitCSV(opts.Type)
	where := ""
	args := []any{opts.Limit, opts.Offset}
	if len(types) > 0 {
		where = " WHERE activity_type = ANY($3)"
		args = append(args, types)
	}

	var total int
	countQ := "SELECT COUNT(*)::int FROM crm_activities"
	if where != "" {
		countQ += " WHERE activity_type = ANY($1)"
		if err := r.db(ctx).QueryRow(ctx, countQ, types).Scan(&total); err != nil {
			return nil, 0, err
		}
	} else if err := r.db(ctx).QueryRow(ctx, countQ).Scan(&total); err != nil {
		return nil, 0, err
	}

	rows, err := r.db(ctx).Query(ctx, `
		SELECT id, activity_type, subject, body, account_id, account_name, contact_id, deal_id, outlet_ref, owner, occurred_at, created_at
		FROM crm_activities`+where+`
		ORDER BY occurred_at DESC LIMIT $1 OFFSET $2
	`, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var out []models.Activity
	for rows.Next() {
		a, err := scanActivity(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, a)
	}
	return out, total, rows.Err()
}

type ActivityInput struct {
	Type       string    `json:"type"`
	Subject    string    `json:"subject"`
	Body       string    `json:"body"`
	Account    string    `json:"account"`
	AccountID  string    `json:"account_id"`
	ContactID  string    `json:"contact_id"`
	DealID     string    `json:"deal_id"`
	OutletRef  string    `json:"outlet_ref"`
	Owner      string    `json:"owner"`
	OccurredAt time.Time `json:"occurred_at"`
}

func (r *Repository) CreateActivity(ctx context.Context, in ActivityInput) (models.Activity, error) {
	id, err := r.NextID(ctx, "ACT", 1000)
	if err != nil {
		return models.Activity{}, err
	}
	if in.OccurredAt.IsZero() {
		in.OccurredAt = time.Now().UTC()
	}
	accountID := in.AccountID
	if accountID == "" && in.Account != "" {
		resolved, err := r.resolveAccountID(ctx, in.Account)
		if err != nil {
			return models.Activity{}, err
		}
		accountID = resolved
	}
	_, err = r.db(ctx).Exec(ctx, `
		INSERT INTO crm_activities (id, activity_type, subject, body, account_id, account_name, contact_id, deal_id, outlet_ref, owner, occurred_at, created_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,NOW())
	`, id, in.Type, in.Subject, in.Body, nullStr(accountID), in.Account, nullStr(in.ContactID), nullStr(in.DealID), nullStr(in.OutletRef), in.Owner, in.OccurredAt)
	if err != nil {
		return models.Activity{}, err
	}
	row := r.db(ctx).QueryRow(ctx, `
		SELECT id, activity_type, subject, body, account_id, account_name, contact_id, deal_id, outlet_ref, owner, occurred_at, created_at
		FROM crm_activities WHERE id = $1
	`, id)
	return scanActivity(row)
}

func scanTicket(row pgx.Row) (models.Ticket, error) {
	var t models.Ticket
	var accountID, contactID, dealID *string
	// outlet_ref and dms_claim_ref are nullable columns. Scanning NULL into a
	// plain string errors, which surfaced as a 500 on GET /tickets whenever any
	// row had no outlet or no DMS claim — the same defect already fixed for
	// activities, which shares this file.
	var outletRef, dmsClaimRef *string
	var sla *time.Time
	err := row.Scan(
		&t.ID, &accountID, &t.Account, &contactID, &outletRef, &dealID, &t.Subject, &t.Type,
		&t.Priority, &t.Channel, &t.Status, &t.Owner, &t.Description, &dmsClaimRef, &sla,
		&t.CreatedAt, &t.UpdatedAt,
	)
	if err != nil {
		return t, err
	}
	if outletRef != nil {
		t.OutletRef = *outletRef
	}
	if dmsClaimRef != nil {
		t.DmsClaimRef = *dmsClaimRef
	}
	if accountID != nil {
		t.AccountID = *accountID
	}
	if contactID != nil {
		t.ContactID = *contactID
	}
	if dealID != nil {
		t.DealID = *dealID
	}
	t.SlaDueAt = sla
	return t, nil
}

func (r *Repository) ListTickets(ctx context.Context, opts ListOpts) ([]models.Ticket, int, error) {
	opts.Limit = clampLimit(opts.Limit)
	where := []string{"1=1"}
	args := []any{}
	i := 1
	where, args = applyScope(opts, "owner", where, args, &i)
	if opts.Status != "" {
		where = append(where, fmt.Sprintf("status = $%d", i))
		args = append(args, opts.Status)
		i++
	}
	whereSQL := strings.Join(where, " AND ")
	var total int
	if err := r.db(ctx).QueryRow(ctx, "SELECT COUNT(*)::int FROM crm_tickets WHERE "+whereSQL, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	args = append(args, opts.Limit, opts.Offset)
	rows, err := r.db(ctx).Query(ctx, `
		SELECT id, account_id, account_name, contact_id, outlet_ref, deal_id, subject, ticket_type, priority, channel,
		       status, owner, description, dms_claim_ref, sla_due_at, created_at, updated_at
		FROM crm_tickets WHERE `+whereSQL+` ORDER BY created_at DESC LIMIT $`+fmt.Sprint(i)+` OFFSET $`+fmt.Sprint(i+1), args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var out []models.Ticket
	for rows.Next() {
		t, err := scanTicket(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, t)
	}
	return out, total, rows.Err()
}

type TicketInput struct {
	Account     string `json:"account"`
	AccountID   string `json:"account_id"`
	ContactID   string `json:"contact_id"`
	OutletRef   string `json:"outlet_ref"`
	DealID      string `json:"deal_id"`
	Subject     string `json:"subject"`
	Type        string `json:"type"`
	Priority    string `json:"priority"`
	Channel     string `json:"channel"`
	Owner       string `json:"owner"`
	Description string `json:"description"`
	Status      string `json:"status"`
}

func (r *Repository) CreateTicket(ctx context.Context, in TicketInput) (models.Ticket, error) {
	id, err := r.NextID(ctx, "TKT", 300)
	if err != nil {
		return models.Ticket{}, err
	}
	if in.Priority == "" {
		in.Priority = "P2"
	}
	status := in.Status
	if status == "" {
		status = "open"
	}
	accountID := in.AccountID
	if accountID == "" && in.Account != "" {
		resolved, err := r.resolveAccountID(ctx, in.Account)
		if err != nil {
			return models.Ticket{}, err
		}
		accountID = resolved
	}
	sla := time.Now().UTC().Add(24 * time.Hour)
	now := time.Now().UTC()
	_, err = r.db(ctx).Exec(ctx, `
		INSERT INTO crm_tickets (id, account_id, account_name, contact_id, outlet_ref, deal_id, subject, ticket_type,
			priority, channel, status, owner, description, sla_due_at, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$15)
	`, id, nullStr(accountID), in.Account, nullStr(in.ContactID), nullStr(in.OutletRef), nullStr(in.DealID),
		in.Subject, in.Type, in.Priority, in.Channel, status, in.Owner, in.Description, sla, now)
	if err != nil {
		return models.Ticket{}, err
	}
	row := r.db(ctx).QueryRow(ctx, `
		SELECT id, account_id, account_name, contact_id, outlet_ref, deal_id, subject, ticket_type, priority, channel,
		       status, owner, description, dms_claim_ref, sla_due_at, created_at, updated_at
		FROM crm_tickets WHERE id = $1
	`, id)
	return scanTicket(row)
}

func (r *Repository) GetTicket(ctx context.Context, id string) (models.Ticket, error) {
	row := r.db(ctx).QueryRow(ctx, `
		SELECT id, account_id, account_name, contact_id, outlet_ref, deal_id, subject, ticket_type, priority, channel,
		       status, owner, description, dms_claim_ref, sla_due_at, created_at, updated_at
		FROM crm_tickets WHERE id = $1
	`, id)
	return scanTicket(row)
}

func (r *Repository) PatchTicket(ctx context.Context, id string, patch map[string]any) (models.Ticket, error) {
	sets := []string{"updated_at = NOW()"}
	args := []any{}
	i := 1
	add := func(col string, val any) {
		sets = append(sets, fmt.Sprintf("%s = $%d", col, i))
		args = append(args, val)
		i++
	}
	for k, col := range map[string]string{
		"subject": "subject", "type": "ticket_type", "priority": "priority",
		"channel": "channel", "status": "status", "owner": "owner", "description": "description",
	} {
		if v, ok := patch[k].(string); ok {
			add(col, v)
		}
	}
	if len(sets) == 1 {
		row := r.db(ctx).QueryRow(ctx, `
			SELECT id, account_id, account_name, contact_id, outlet_ref, deal_id, subject, ticket_type, priority, channel,
			       status, owner, description, dms_claim_ref, sla_due_at, created_at, updated_at
			FROM crm_tickets WHERE id = $1
		`, id)
		return scanTicket(row)
	}
	args = append(args, id)
	_, err := r.db(ctx).Exec(ctx, `UPDATE crm_tickets SET `+strings.Join(sets, ", ")+` WHERE id = $`+fmt.Sprint(i), args...)
	if err != nil {
		return models.Ticket{}, err
	}
	row := r.db(ctx).QueryRow(ctx, `
		SELECT id, account_id, account_name, contact_id, outlet_ref, deal_id, subject, ticket_type, priority, channel,
		       status, owner, description, dms_claim_ref, sla_due_at, created_at, updated_at
		FROM crm_tickets WHERE id = $1
	`, id)
	return scanTicket(row)
}

func scanCampaign(row pgx.Row) (models.Campaign, error) {
	var c models.Campaign
	var starts, ends *time.Time
	err := row.Scan(
		&c.ID, &c.Name, &c.Type, &c.Audience, &starts, &ends, &c.BudgetUSD, &c.Owner, &c.Goal, &c.Status,
		&c.CreatedAt, &c.UpdatedAt,
	)
	if err != nil {
		return c, err
	}
	c.StartsOn = starts
	c.EndsOn = ends
	return c, nil
}

func (r *Repository) ListCampaigns(ctx context.Context, opts ListOpts) ([]models.Campaign, int, error) {
	opts.Limit = clampLimit(opts.Limit)
	var total int
	if err := r.db(ctx).QueryRow(ctx, `SELECT COUNT(*)::int FROM crm_campaigns`).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := r.db(ctx).Query(ctx, `
		SELECT id, name, campaign_type, audience, starts_on, ends_on, budget_usd, owner, goal, status, created_at, updated_at
		FROM crm_campaigns ORDER BY created_at DESC LIMIT $1 OFFSET $2
	`, opts.Limit, opts.Offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var out []models.Campaign
	for rows.Next() {
		c, err := scanCampaign(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, c)
	}
	return out, total, rows.Err()
}

type CampaignInput struct {
	Name      string     `json:"name"`
	Type      string     `json:"type"`
	Audience  string     `json:"audience"`
	StartsOn  *time.Time `json:"starts_on"`
	EndsOn    *time.Time `json:"ends_on"`
	BudgetUSD float64    `json:"budget_usd"`
	Owner     string     `json:"owner"`
	Goal      string     `json:"goal"`
	Status    string     `json:"status"`
}

func (r *Repository) GetCampaign(ctx context.Context, id string) (models.Campaign, error) {
	row := r.db(ctx).QueryRow(ctx, `
		SELECT id, name, campaign_type, audience, starts_on, ends_on, budget_usd, owner, goal, status, created_at, updated_at
		FROM crm_campaigns WHERE id = $1
	`, id)
	return scanCampaign(row)
}

func (r *Repository) PatchCampaign(ctx context.Context, id string, patch map[string]any) (models.Campaign, error) {
	sets := []string{"updated_at = NOW()"}
	args := []any{}
	i := 1
	add := func(col string, val any) {
		sets = append(sets, fmt.Sprintf("%s = $%d", col, i))
		args = append(args, val)
		i++
	}
	for k, col := range map[string]string{
		"name": "name", "type": "campaign_type", "audience": "audience",
		"status": "status", "owner": "owner", "goal": "goal",
	} {
		if v, ok := patch[k].(string); ok {
			add(col, v)
		}
	}
	if v, ok := patch["budget_usd"].(float64); ok {
		add("budget_usd", v)
	}
	if len(sets) == 1 {
		return r.GetCampaign(ctx, id)
	}
	args = append(args, id)
	_, err := r.db(ctx).Exec(ctx, `UPDATE crm_campaigns SET `+strings.Join(sets, ", ")+` WHERE id = $`+fmt.Sprint(i), args...)
	if err != nil {
		return models.Campaign{}, err
	}
	return r.GetCampaign(ctx, id)
}

func (r *Repository) CreateCampaign(ctx context.Context, in CampaignInput) (models.Campaign, error) {
	id, err := r.NextID(ctx, "CMP", 100)
	if err != nil {
		return models.Campaign{}, err
	}
	now := time.Now().UTC()
	status := in.Status
	if status == "" {
		status = "draft"
	}
	_, err = r.db(ctx).Exec(ctx, `
		INSERT INTO crm_campaigns (id, name, campaign_type, audience, starts_on, ends_on, budget_usd, owner, goal, status, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$11)
	`, id, in.Name, in.Type, in.Audience, in.StartsOn, in.EndsOn, nullFloat(in.BudgetUSD), in.Owner, in.Goal, status, now)
	if err != nil {
		return models.Campaign{}, err
	}
	row := r.db(ctx).QueryRow(ctx, `
		SELECT id, name, campaign_type, audience, starts_on, ends_on, budget_usd, owner, goal, status, created_at, updated_at
		FROM crm_campaigns WHERE id = $1
	`, id)
	return scanCampaign(row)
}
