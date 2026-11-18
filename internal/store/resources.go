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
	where := []string{"1=1"}
	args := []any{}
	i := 1
	where, args = applyScope(opts, "owner", where, args, &i)
	if opts.AccountID != "" {
		where = append(where, fmt.Sprintf("account_id = $%d", i))
		args = append(args, opts.AccountID)
		i++
	}
	whereSQL := strings.Join(where, " AND ")
	var total int
	if err := r.db(ctx).QueryRow(ctx, "SELECT COUNT(*)::int FROM crm_quotes WHERE "+whereSQL, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	args = append(args, opts.Limit, opts.Offset)
	rows, err := r.db(ctx).Query(ctx, `
		SELECT id, ref, account_id, account_name, deal_id, template, currency, incoterms, payment_terms,
		       valid_until, total, status, version, owner, line_items, finance_ar_ref, contract_ref, created_at, updated_at
		FROM crm_quotes WHERE `+whereSQL+` ORDER BY created_at DESC LIMIT $`+fmt.Sprint(i)+` OFFSET $`+fmt.Sprint(i+1), args...)
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
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	r.fillQuoteLinks(ctx, out)
	return out, total, nil
}

type QuoteInput struct {
	Ref          string                 `json:"ref"`
	Account      string                 `json:"account"`
	AccountID    string                 `json:"account_id"`
	DealID       string                 `json:"deal_id"`
	// Deal is a name resolved to DealID when that is blank.
	Deal         string                 `json:"deal"`
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
	id := r.NewID()
	if in.Ref == "" {
		in.Ref = id
	}
	if in.Currency == "" {
		in.Currency = "USD"
	}
	accountID, accountName, err := r.requireAccountRef(ctx, in.AccountID, in.Account)
	if err != nil {
		return models.Quote{}, err
	}
	dealID := in.DealID
	if dealID == "" && in.Deal != "" {
		resolved, err := r.resolveDealID(ctx, in.Deal)
		if err != nil {
			return models.Quote{}, err
		}
		dealID = resolved
	}
	if len(in.LineItems) > 0 {
		in.Total = models.QuoteLinesTotal(in.LineItems)
	}
	lineRaw, _ := json.Marshal(in.LineItems)
	now := time.Now().UTC()
	_, err = r.db(ctx).Exec(ctx, `
		INSERT INTO crm_quotes (id, ref, account_id, account_name, deal_id, template, currency, incoterms, payment_terms,
			valid_until, total, status, version, owner, line_items, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,'draft',1,$12,$13,$14,$14)
	`, id, in.Ref, accountID, accountName, nullStr(dealID), in.Template, in.Currency,
		in.Incoterms, in.PaymentTerms, in.ValidUntil, in.Total, in.Owner, lineRaw, now)
	if err != nil {
		return models.Quote{}, err
	}
	row := r.db(ctx).QueryRow(ctx, `
		SELECT id, ref, account_id, account_name, deal_id, template, currency, incoterms, payment_terms,
		       valid_until, total, status, version, owner, line_items, finance_ar_ref, contract_ref, created_at, updated_at
		FROM crm_quotes WHERE id = $1
	`, id)
	return r.quoteWithLinks(ctx, row)
}

func (r *Repository) GetQuote(ctx context.Context, id string) (models.Quote, error) {
	row := r.db(ctx).QueryRow(ctx, `
		SELECT id, ref, account_id, account_name, deal_id, template, currency, incoterms, payment_terms,
		       valid_until, total, status, version, owner, line_items, finance_ar_ref, contract_ref, created_at, updated_at
		FROM crm_quotes WHERE id = $1
	`, id)
	return r.quoteWithLinks(ctx, row)
}

func (r *Repository) PatchQuote(ctx context.Context, id string, patch map[string]any) (models.Quote, error) {
	sets := []string{}
	args := []any{id}
	i := 2
	for _, field := range []struct{ key, col string }{
		{"status", "status"}, {"total", "total"}, {"currency", "currency"},
		{"incoterms", "incoterms"}, {"payment_terms", "payment_terms"}, {"owner", "owner"},
		{"template", "template"}, {"ref", "ref"},
	} {
		if v, ok := patch[field.key]; ok {
			sets = append(sets, fmt.Sprintf("%s = $%d", field.col, i))
			args = append(args, v)
			i++
		}
	}
	if raw, ok := patch["line_items"]; ok {
		items := normalizeQuoteLines(raw)
		lineRaw, _ := json.Marshal(items)
		sets = append(sets, fmt.Sprintf("line_items = $%d", i))
		args = append(args, lineRaw)
		i++
		if _, hasTotal := patch["total"]; !hasTotal && len(items) > 0 {
			sets = append(sets, fmt.Sprintf("total = $%d", i))
			args = append(args, models.QuoteLinesTotal(items))
			i++
		}
	}
	if v, ok := patch["account"].(string); ok {
		accountID, accountName, err := r.requireAccountRef(ctx, "", v)
		if err != nil {
			return models.Quote{}, err
		}
		sets = append(sets, fmt.Sprintf("account_id = $%d", i))
		args = append(args, accountID)
		i++
		sets = append(sets, fmt.Sprintf("account_name = $%d", i))
		args = append(args, accountName)
		i++
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

func normalizeQuoteLines(raw any) []models.QuoteLineItem {
	switch v := raw.(type) {
	case []models.QuoteLineItem:
		return v
	case []any:
		out := make([]models.QuoteLineItem, 0, len(v))
		for _, row := range v {
			m, ok := row.(map[string]any)
			if !ok {
				continue
			}
			item := models.QuoteLineItem{
				Description: fmt.Sprint(m["description"]),
				LotRef:      fmt.Sprint(m["lot_ref"]),
				Unit:        fmt.Sprint(m["unit"]),
			}
			switch q := m["quantity"].(type) {
			case float64:
				item.Quantity = int(q)
			case int:
				item.Quantity = q
			case json.Number:
				n, _ := q.Int64()
				item.Quantity = int(n)
			}
			switch p := m["price"].(type) {
			case float64:
				item.Price = p
			case int:
				item.Price = float64(p)
			case json.Number:
				item.Price, _ = p.Float64()
			}
			out = append(out, item)
		}
		return out
	default:
		b, err := json.Marshal(raw)
		if err != nil {
			return nil
		}
		var out []models.QuoteLineItem
		_ = json.Unmarshal(b, &out)
		return out
	}
}

func scanActivity(row pgx.Row) (models.Activity, error) {
	var a models.Activity
	var accountID, contactID, dealID, outletRef *string
	var dueAt *time.Time
	var attrs []byte
	err := row.Scan(
		&a.ID, &a.Type, &a.Subject, &a.Body, &accountID, &a.Account, &contactID, &dealID,
		&outletRef, &a.Owner, &a.OccurredAt, &dueAt, &a.Status, &attrs, &a.CreatedAt,
	)
	a.DueAt = dueAt
	a.Attrs = decodeAttrs(attrs)
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
	where := []string{"1=1"}
	args := []any{}
	i := 1
	// Activities were the one collection with no owner scope, so a sales_rep
	// saw every follow-up in the company. The scope is wider than the other
	// tables' `owner = me` on purpose: a rep's follow-ups are often logged by
	// someone else about the rep's account (a manager's call note, a support
	// visit), and hiding those would make the rep's own 360 lie to them. So a
	// scoped caller sees what they own OR what is on an account they own.
	where, args = applyActivityScope(opts, where, args, &i)
	if opts.AccountID != "" {
		where = append(where, fmt.Sprintf("account_id = $%d", i))
		args = append(args, opts.AccountID)
		i++
	}
	if opts.DealID != "" {
		where = append(where, fmt.Sprintf("deal_id = $%d", i))
		args = append(args, opts.DealID)
		i++
	}
	if opts.ContactID != "" {
		where = append(where, fmt.Sprintf("contact_id = $%d", i))
		args = append(args, opts.ContactID)
		i++
	}
	if opts.LeadID != "" {
		where = append(where, fmt.Sprintf("attrs->>'lead_id' = $%d", i))
		args = append(args, opts.LeadID)
		i++
	}
	if len(types) > 0 {
		where = append(where, fmt.Sprintf("activity_type = ANY($%d)", i))
		args = append(args, types)
		i++
	}
	whereSQL := strings.Join(where, " AND ")

	var total int
	if err := r.db(ctx).QueryRow(ctx, "SELECT COUNT(*)::int FROM crm_activities WHERE "+whereSQL, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	args = append(args, opts.Limit, opts.Offset)
	rows, err := r.db(ctx).Query(ctx, `
		SELECT id, activity_type, subject, body, account_id, account_name, contact_id, deal_id, outlet_ref, owner, occurred_at, due_at, status, attrs, created_at
		FROM crm_activities WHERE `+whereSQL+`
		ORDER BY occurred_at DESC LIMIT $`+fmt.Sprint(i)+` OFFSET $`+fmt.Sprint(i+1)+`
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
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	// Names for contact_id / deal_id. Dropped once already when the scope
	// change rewrote this tail, which left every list read blank on the links
	// the single-record read still carried.
	r.fillActivityLinks(ctx, out)
	return out, total, nil
}

type ActivityInput struct {
	Type       string    `json:"type"`
	Subject    string    `json:"subject"`
	Body       string    `json:"body"`
	Account    string    `json:"account"`
	AccountID  string    `json:"account_id"`
	ContactID  string    `json:"contact_id"`
	DealID     string    `json:"deal_id"`
	// Contact and Deal are names, resolved to the ids above when those are
	// blank — the same convention as Account. The record clients only have
	// strings.
	Contact    string    `json:"contact"`
	Deal       string    `json:"deal"`
	OutletRef  string    `json:"outlet_ref"`
	Owner      string    `json:"owner"`
	OccurredAt time.Time `json:"occurred_at"`
	// DueAt and Status make a follow-up a follow-up: an activity with neither
	// cannot be chased or closed. Promoted columns rather than attrs, because
	// the engagement views order and filter on them.
	DueAt  *time.Time `json:"due_at"`
	Status string     `json:"status"`
	// Attrs carries client fields with no promoted column (reference, the
	// free-text "related to"). See db/migrations/0008_entity_attrs.sql.
	Attrs map[string]any `json:"attrs"`
}

func (r *Repository) CreateActivity(ctx context.Context, in ActivityInput) (models.Activity, error) {
	id := r.NewID()
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
	contactID, dealID, err := r.resolveLinks(ctx, in.ContactID, in.Contact, in.DealID, in.Deal)
	if err != nil {
		return models.Activity{}, err
	}
	if in.Status == "" {
		in.Status = models.ActivityStatusPlanned
	}
	_, err = r.db(ctx).Exec(ctx, `
		INSERT INTO crm_activities (id, activity_type, subject, body, account_id, account_name, contact_id, deal_id, outlet_ref, owner, occurred_at, due_at, status, attrs, created_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,NOW())
	`, id, in.Type, in.Subject, in.Body, nullStr(accountID), in.Account, nullStr(contactID), nullStr(dealID), nullStr(in.OutletRef), in.Owner, in.OccurredAt,
		in.DueAt, in.Status, encodeAttrs(in.Attrs))
	if err != nil {
		return models.Activity{}, err
	}
	row := r.db(ctx).QueryRow(ctx, `
		SELECT id, activity_type, subject, body, account_id, account_name, contact_id, deal_id, outlet_ref, owner, occurred_at, due_at, status, attrs, created_at
		FROM crm_activities WHERE id = $1
	`, id)
	a, err := scanActivity(row)
	if err != nil {
		return a, err
	}
	r.fillActivityLinks(ctx, []models.Activity{a})
	return r.GetActivity(ctx, id)
}

func scanTicket(row pgx.Row) (models.Ticket, error) {
	var t models.Ticket
	var accountID, contactID, dealID *string
	// outlet_ref and dms_claim_ref are nullable columns. Scanning NULL into a
	// plain string errors, which surfaced as a 500 on GET /tickets whenever any
	// row had no outlet or no DMS claim — the same defect already fixed for
	// activities, which shares this file.
	var outletRef, dmsClaimRef *string
	var occurredAt, sla, resolvedAt *time.Time
	var attrs []byte
	err := row.Scan(
		&t.ID, &accountID, &t.Account, &contactID, &outletRef, &dealID, &t.Subject, &t.Type,
		&t.Priority, &t.Channel, &t.Status, &t.Owner, &t.Description, &dmsClaimRef, &occurredAt, &sla,
		&resolvedAt, &t.Resolution, &attrs, &t.CreatedAt, &t.UpdatedAt,
	)
	if err != nil {
		return t, err
	}
	t.OccurredAt = occurredAt
	t.ResolvedAt = resolvedAt
	t.Attrs = decodeAttrs(attrs)
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
	if opts.AccountID != "" {
		where = append(where, fmt.Sprintf("account_id = $%d", i))
		args = append(args, opts.AccountID)
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
		       status, owner, description, dms_claim_ref, occurred_at, sla_due_at, resolved_at, resolution, attrs, created_at, updated_at
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
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	r.fillTicketLinks(ctx, out)
	return out, total, nil
}

type TicketInput struct {
	Account     string `json:"account"`
	AccountID   string `json:"account_id"`
	ContactID   string `json:"contact_id"`
	OutletRef   string `json:"outlet_ref"`
	DealID      string `json:"deal_id"`
	// Names resolved to contact_id / deal_id when those are blank; see
	// ActivityInput.
	Contact     string `json:"contact"`
	Deal        string `json:"deal"`
	Subject     string `json:"subject"`
	Type        string `json:"type"`
	Priority    string `json:"priority"`
	Channel     string `json:"channel"`
	Owner       string `json:"owner"`
	Description string `json:"description"`
	Status      string `json:"status"`
	// OccurredAt is when the incident happened. Absent, CreateTicket falls back
	// to now — which is what every row got before 0012, and why a complaint could
	// not be logged for the day it actually happened.
	OccurredAt *time.Time `json:"occurred_at"`
	// A complaint closed with no recorded outcome is the standard support audit
	// finding, so the resolution and when it happened are promoted columns.
	ResolvedAt *time.Time `json:"resolved_at"`
	Resolution string     `json:"resolution"`
	// Attrs carries client fields with no promoted column (reference, the
	// related order/invoice). See db/migrations/0008_entity_attrs.sql.
	Attrs map[string]any `json:"attrs"`
}

func (r *Repository) CreateTicket(ctx context.Context, in TicketInput) (models.Ticket, error) {
	id := r.NewID()
	if in.Priority == "" {
		in.Priority = "P2"
	}
	status := in.Status
	if status == "" {
		status = "open"
	}
	accountID, accountName, err := r.requireAccountRef(ctx, in.AccountID, in.Account)
	if err != nil {
		return models.Ticket{}, err
	}
	contactID, dealID, err := r.resolveLinks(ctx, in.ContactID, in.Contact, in.DealID, in.Deal)
	if err != nil {
		return models.Ticket{}, err
	}
	if err := r.requireContactBelongsToAccount(ctx, contactID, accountID); err != nil {
		return models.Ticket{}, err
	}
	// The SLA clock runs from when the incident happened, not from when it
	// was typed in, and its length depends on priority (models.TicketSLA).
	now := time.Now().UTC()
	occurredAt := in.OccurredAt
	if occurredAt == nil {
		occurredAt = &now
	}
	sla := occurredAt.Add(models.SLAFor(in.Priority))
	_, err = r.db(ctx).Exec(ctx, `
		INSERT INTO crm_tickets (id, account_id, account_name, contact_id, outlet_ref, deal_id, subject, ticket_type,
			priority, channel, status, owner, description, occurred_at, sla_due_at, resolved_at, resolution, attrs, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$19)
	`, id, accountID, accountName, nullStr(contactID), nullStr(in.OutletRef), nullStr(dealID),
		in.Subject, in.Type, in.Priority, in.Channel, status, in.Owner, in.Description, occurredAt, sla,
		in.ResolvedAt, in.Resolution, encodeAttrs(in.Attrs), now)
	if err != nil {
		return models.Ticket{}, err
	}
	row := r.db(ctx).QueryRow(ctx, `
		SELECT id, account_id, account_name, contact_id, outlet_ref, deal_id, subject, ticket_type, priority, channel,
		       status, owner, description, dms_claim_ref, occurred_at, sla_due_at, resolved_at, resolution, attrs, created_at, updated_at
		FROM crm_tickets WHERE id = $1
	`, id)
	return r.ticketWithLinks(ctx, row)
}

func (r *Repository) GetTicket(ctx context.Context, id string) (models.Ticket, error) {
	row := r.db(ctx).QueryRow(ctx, `
		SELECT id, account_id, account_name, contact_id, outlet_ref, deal_id, subject, ticket_type, priority, channel,
		       status, owner, description, dms_claim_ref, occurred_at, sla_due_at, resolved_at, resolution, attrs, created_at, updated_at
		FROM crm_tickets WHERE id = $1
	`, id)
	return r.ticketWithLinks(ctx, row)
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
		// The outcome of a complaint was unwritable: an agent could close a
		// ticket but never record how.
		"resolution": "resolution",
		// account is applied below via requireAccountRef so account_id and
		// account_name stay in lockstep (and we never SET the same column twice).
	} {
		if v, ok := patch[k].(string); ok {
			add(col, v)
		}
	}
	if v, ok := patch["occurred_at"]; ok {
		add("occurred_at", parsePatchTime(v))
	}
	if v, ok := patch["resolved_at"]; ok {
		add("resolved_at", parsePatchTime(v))
	}
	if v, ok := patch["sla_due_at"]; ok {
		add("sla_due_at", parsePatchTime(v))
	}
	// A priority change re-derives the SLA unless the caller set one
	// explicitly in the same request.
	if v, ok := patch["priority"].(string); ok {
		if _, explicit := patch["sla_due_at"]; !explicit {
			sets = append(sets, fmt.Sprintf("sla_due_at = COALESCE(occurred_at, created_at) + $%d::interval", i))
			args = append(args, models.SLAFor(v).String())
			i++
		}
	}
	cur, err := r.GetTicket(ctx, id)
	if err != nil {
		return models.Ticket{}, err
	}
	accountIDForCheck := cur.AccountID
	contactIDForCheck := cur.ContactID
	if i, err = r.patchLinks(ctx, patch, &sets, &args, i); err != nil {
		return models.Ticket{}, err
	}
	if v, ok := patch["contact_id"].(string); ok {
		contactIDForCheck = strings.TrimSpace(v)
	} else if v, ok := patch["contact"].(string); ok {
		if resolved, rerr := r.resolveContactID(ctx, v); rerr != nil {
			return models.Ticket{}, rerr
		} else {
			contactIDForCheck = resolved
		}
	}
	// Keep the foreign key with the name, for the reason PatchDeal spells out:
	// a display name and an account_id that disagree look correct on screen and
	// are wrong in every join. Tickets must stay on a real customer.
	if v, ok := patch["account"].(string); ok {
		accountID, accountName, err := r.requireAccountRef(ctx, "", v)
		if err != nil {
			return models.Ticket{}, err
		}
		accountIDForCheck = accountID
		add("account_id", accountID)
		add("account_name", accountName)
	}
	if err := r.requireContactBelongsToAccount(ctx, contactIDForCheck, accountIDForCheck); err != nil {
		return models.Ticket{}, err
	}
	if attrs, ok := patchAttrs(patch); ok {
		sets = append(sets, attrs.SetExpr(i))
		args = append(args, attrs.Arg())
		i++
	}
	if len(sets) == 1 {
		return cur, nil
	}
	args = append(args, id)
	_, err = r.db(ctx).Exec(ctx, `UPDATE crm_tickets SET `+strings.Join(sets, ", ")+` WHERE id = $`+fmt.Sprint(i), args...)
	if err != nil {
		return models.Ticket{}, err
	}
	row := r.db(ctx).QueryRow(ctx, `
		SELECT id, account_id, account_name, contact_id, outlet_ref, deal_id, subject, ticket_type, priority, channel,
		       status, owner, description, dms_claim_ref, occurred_at, sla_due_at, resolved_at, resolution, attrs, created_at, updated_at
		FROM crm_tickets WHERE id = $1
	`, id)
	return r.ticketWithLinks(ctx, row)
}

func scanCampaign(row pgx.Row) (models.Campaign, error) {
	var c models.Campaign
	var starts, ends *time.Time
	var segmentID *string
	// budget_usd is nullable and CreateCampaign writes NULL for it whenever no
	// budget was given (nullFloat turns 0 into nil). Scanning that into a plain
	// float64 errors, and because the create re-reads the row it just inserted,
	// EVERY campaign created without a budget failed with 500 "create campaign
	// failed" — after the insert had already succeeded, so the transaction rolled
	// it back and nothing was left to show for it. The seeded campaigns all carry
	// a budget, which is why every read path looked healthy.
	var budget *float64
	err := row.Scan(
		&c.ID, &c.Name, &c.Type, &c.Audience, &segmentID, &starts, &ends, &budget, &c.Owner, &c.Goal, &c.Status,
		&c.CreatedAt, &c.UpdatedAt,
	)
	if err != nil {
		return c, err
	}
	if segmentID != nil {
		c.SegmentID = *segmentID
	}
	if budget != nil {
		c.BudgetUSD = *budget
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
		SELECT id, name, campaign_type, audience, segment_id, starts_on, ends_on, budget_usd, owner, goal, status, created_at, updated_at
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
	SegmentID string     `json:"segment_id"`
	StartsOn  *time.Time `json:"starts_on"`
	EndsOn    *time.Time `json:"ends_on"`
	BudgetUSD float64    `json:"budget_usd"`
	Owner     string     `json:"owner"`
	Goal      string     `json:"goal"`
	Status    string     `json:"status"`
}

func (r *Repository) GetCampaign(ctx context.Context, id string) (models.Campaign, error) {
	row := r.db(ctx).QueryRow(ctx, `
		SELECT id, name, campaign_type, audience, segment_id, starts_on, ends_on, budget_usd, owner, goal, status, created_at, updated_at
		FROM crm_campaigns WHERE id = $1
	`, id)
	return scanCampaign(row)
}

func (r *Repository) PatchCampaign(ctx context.Context, id string, patch map[string]any) (models.Campaign, error) {
	cur, err := r.GetCampaign(ctx, id)
	if err != nil {
		return models.Campaign{}, err
	}
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
		"owner": "owner", "goal": "goal",
	} {
		if v, ok := patch[k].(string); ok {
			add(col, v)
		}
	}
	if v, ok := patch["segment_id"].(string); ok {
		v = strings.TrimSpace(v)
		if v == "" {
			add("segment_id", nil)
		} else {
			var exists string
			if err := r.db(ctx).QueryRow(ctx, `SELECT id FROM crm_segments WHERE id = $1`, v).Scan(&exists); err != nil {
				return models.Campaign{}, fmt.Errorf("%w: segment %q not found", ErrInvalidInput, v)
			}
			add("segment_id", v)
			if _, hasAud := patch["audience"]; !hasAud {
				var segName string
				_ = r.db(ctx).QueryRow(ctx, `SELECT name FROM crm_segments WHERE id = $1`, v).Scan(&segName)
				if segName != "" {
					add("audience", segName)
				}
			}
		}
	} else if aud, ok := patch["audience"].(string); ok {
		aud = strings.TrimSpace(aud)
		if aud == "" {
			add("segment_id", nil)
		} else {
			var foundID string
			if err := r.db(ctx).QueryRow(ctx,
				`SELECT id FROM crm_segments WHERE lower(name) = lower($1) LIMIT 1`, aud,
			).Scan(&foundID); err == nil {
				add("segment_id", foundID)
			}
		}
	}
	if raw, ok := patch["status"].(string); ok {
		next := models.NormalizeCampaignStatus(raw)
		if !models.InVocab(next, models.CampaignStatuses) {
			return models.Campaign{}, fmt.Errorf("%w: invalid campaign status %q", ErrInvalidInput, raw)
		}
		from := models.NormalizeCampaignStatus(cur.Status)
		if !models.CanTransitionCampaign(from, next) {
			return models.Campaign{}, fmt.Errorf("%w: cannot move campaign from %s to %s", ErrConflict, from, next)
		}
		add("status", next)
	}
	if v, ok := patch["budget_usd"].(float64); ok {
		add("budget_usd", v)
	}
	if len(sets) == 1 {
		return cur, nil
	}
	args = append(args, id)
	_, err = r.db(ctx).Exec(ctx, `UPDATE crm_campaigns SET `+strings.Join(sets, ", ")+` WHERE id = $`+fmt.Sprint(i), args...)
	if err != nil {
		return models.Campaign{}, err
	}
	return r.GetCampaign(ctx, id)
}

func (r *Repository) CreateCampaign(ctx context.Context, in CampaignInput) (models.Campaign, error) {
	id := r.NewID()
	now := time.Now().UTC()
	// Creates always start as draft — CEO approval and go-live happen via PATCH.
	status := models.CampaignStatusDraft
	if raw := strings.TrimSpace(in.Status); raw != "" {
		if models.NormalizeCampaignStatus(raw) != models.CampaignStatusDraft {
			return models.Campaign{}, fmt.Errorf("%w: new campaigns must start as draft (got %q)", ErrInvalidInput, raw)
		}
	}
	segmentID := strings.TrimSpace(in.SegmentID)
	audience := strings.TrimSpace(in.Audience)
	if segmentID != "" {
		var segName string
		if err := r.db(ctx).QueryRow(ctx, `SELECT name FROM crm_segments WHERE id = $1`, segmentID).Scan(&segName); err != nil {
			return models.Campaign{}, fmt.Errorf("%w: segment %q not found", ErrInvalidInput, segmentID)
		}
		if audience == "" {
			audience = segName
		}
	} else if audience != "" {
		// Desk audience picker stores the segment name — resolve to FK when possible.
		var foundID string
		if err := r.db(ctx).QueryRow(ctx,
			`SELECT id FROM crm_segments WHERE lower(name) = lower($1) LIMIT 1`, audience,
		).Scan(&foundID); err == nil {
			segmentID = foundID
		}
	}
	_, err := r.db(ctx).Exec(ctx, `
		INSERT INTO crm_campaigns (id, name, campaign_type, audience, segment_id, starts_on, ends_on, budget_usd, owner, goal, status, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$12)
	`, id, in.Name, in.Type, audience, nullStr(segmentID), in.StartsOn, in.EndsOn, nullFloat(in.BudgetUSD), in.Owner, in.Goal, status, now)
	if err != nil {
		return models.Campaign{}, err
	}
	row := r.db(ctx).QueryRow(ctx, `
		SELECT id, name, campaign_type, audience, segment_id, starts_on, ends_on, budget_usd, owner, goal, status, created_at, updated_at
		FROM crm_campaigns WHERE id = $1
	`, id)
	return scanCampaign(row)
}

// ticketWithLinks scans one ticket and fills its contact/deal names.
func (r *Repository) ticketWithLinks(ctx context.Context, row pgx.Row) (models.Ticket, error) {
	t, err := scanTicket(row)
	if err != nil {
		return t, err
	}
	one := []models.Ticket{t}
	r.fillTicketLinks(ctx, one)
	return one[0], nil
}

func (r *Repository) fillQuoteLinks(ctx context.Context, items []models.Quote) {
	var dealIDs []string
	for _, q := range items {
		if q.DealID != "" {
			dealIDs = append(dealIDs, q.DealID)
		}
	}
	deals, _ := r.linkNames(ctx, "crm_deals", dealIDs)
	for i := range items {
		items[i].DealName = deals[items[i].DealID]
	}
}

func (r *Repository) quoteWithLinks(ctx context.Context, row pgx.Row) (models.Quote, error) {
	q, err := scanQuote(row)
	if err != nil {
		return q, err
	}
	one := []models.Quote{q}
	r.fillQuoteLinks(ctx, one)
	return one[0], nil
}
