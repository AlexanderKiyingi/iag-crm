package store

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/iag/crm/backend/internal/models"
)

func scanDeal(row pgx.Row) (models.Deal, error) {
	var d models.Deal
	var accountID *string
	var closeDate *time.Time
	var financeAR *string
	var attrs []byte
	err := row.Scan(
		&d.ID, &d.Name, &accountID, &d.Account, &d.Stage, &d.Probability, &d.Owner,
		&d.Currency, &d.Amount, &d.AmountDisplay, &d.Description, &d.Source, &d.DmsLinked,
		&closeDate, &d.Notes, &financeAR, &attrs, &d.CreatedAt, &d.UpdatedAt,
	)
	d.Attrs = decodeAttrs(attrs)
	if financeAR != nil {
		d.FinanceARRef = *financeAR
	}
	if err != nil {
		return d, err
	}
	if accountID != nil {
		d.AccountID = *accountID
	}
	d.CloseDate = closeDate
	d.AgeDays = int(time.Since(d.CreatedAt).Hours() / 24)
	if d.AmountDisplay == "" {
		d.AmountDisplay = formatAmount(d.Currency, d.Amount)
	}
	return d, nil
}

func formatAmount(currency string, amount float64) string {
	if currency == "UGX" {
		if amount >= 1_000_000 {
			return fmt.Sprintf("UGX %.0fM", amount/1_000_000)
		}
		return fmt.Sprintf("UGX %.0f", amount)
	}
	if amount >= 1000 {
		return fmt.Sprintf("$%.0fK", amount/1000)
	}
	return fmt.Sprintf("$%.0f", amount)
}

func (r *Repository) ListDeals(ctx context.Context, opts ListOpts) ([]models.Deal, int, error) {
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
	if opts.Stage != "" {
		where = append(where, fmt.Sprintf("stage = $%d", i))
		args = append(args, opts.Stage)
		i++
	}
	whereSQL := strings.Join(where, " AND ")
	var total int
	if err := r.db(ctx).QueryRow(ctx, "SELECT COUNT(*)::int FROM crm_deals WHERE "+whereSQL, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	args = append(args, opts.Limit, opts.Offset)
	rows, err := r.db(ctx).Query(ctx, `
		SELECT id, name, account_id, account_name, stage, probability, owner, currency, amount, amount_display,
		       description, source, dms_linked, close_date, notes, finance_ar_ref, attrs, created_at, updated_at
		FROM crm_deals WHERE `+whereSQL+` ORDER BY updated_at DESC LIMIT $`+fmt.Sprint(i)+` OFFSET $`+fmt.Sprint(i+1), args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var out []models.Deal
	for rows.Next() {
		d, err := scanDeal(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, d)
	}
	return out, total, rows.Err()
}

func (r *Repository) GetDeal(ctx context.Context, id string) (models.Deal, error) {
	row := r.db(ctx).QueryRow(ctx, `
		SELECT id, name, account_id, account_name, stage, probability, owner, currency, amount, amount_display,
		       description, source, dms_linked, close_date, notes, finance_ar_ref, attrs, created_at, updated_at
		FROM crm_deals WHERE id = $1
	`, id)
	return scanDeal(row)
}

type DealInput struct {
	Name          string     `json:"name"`
	Account       string     `json:"account"`
	AccountID     string     `json:"account_id"`
	Stage         string     `json:"stage"`
	Probability   int        `json:"probability"`
	Owner         string     `json:"owner"`
	Currency      string     `json:"currency"`
	Amount        float64    `json:"amount"`
	AmountDisplay string     `json:"amount_display"`
	Description   string     `json:"description"`
	Source        string     `json:"source"`
	DmsLinked     bool       `json:"dms_linked"`
	CloseDate     *time.Time `json:"close_date"`
	Notes         string     `json:"notes"`
	// Attrs carries client fields with no promoted column (linked sales order,
	// …). See db/migrations/0008_entity_attrs.sql.
	Attrs map[string]any `json:"attrs"`
}

func (r *Repository) CreateDeal(ctx context.Context, in DealInput) (models.Deal, error) {
	id := r.NewID()
	accountID := in.AccountID
	accountName := in.Account
	if accountID == "" && accountName != "" {
		aid, err := r.resolveAccountID(ctx, accountName)
		if err != nil {
			return models.Deal{}, err
		}
		accountID = aid
	}
	if in.Stage == "" {
		in.Stage = models.DealStageLead
	}
	if in.Currency == "" {
		in.Currency = "USD"
	}
	if in.Probability == 0 {
		in.Probability = stageDefaultProbability(in.Stage)
	}
	if in.AmountDisplay == "" {
		in.AmountDisplay = formatAmount(in.Currency, in.Amount)
	}
	now := time.Now().UTC()
	_, err := r.db(ctx).Exec(ctx, `
		INSERT INTO crm_deals (
			id, name, account_id, account_name, stage, probability, owner, currency, amount, amount_display,
			description, source, dms_linked, close_date, notes, attrs, created_at, updated_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$17)
	`, id, in.Name, nullStr(accountID), accountName, in.Stage, in.Probability, in.Owner,
		in.Currency, in.Amount, in.AmountDisplay, in.Description, in.Source, in.DmsLinked, in.CloseDate, in.Notes,
		encodeAttrs(in.Attrs), now)
	if err != nil {
		return models.Deal{}, err
	}
	return r.GetDeal(ctx, id)
}

func stageDefaultProbability(stage string) int {
	switch stage {
	case models.DealStageLead:
		return 18
	case models.DealStageQualified:
		return 38
	case models.DealStageProposal:
		return 58
	case models.DealStageNegotiation:
		return 75
	case models.DealStageWon:
		return 100
	default:
		return 20
	}
}

func (r *Repository) PatchDeal(ctx context.Context, id string, patch map[string]any) (models.Deal, error) {
	sets := []string{"updated_at = NOW()"}
	args := []any{}
	i := 1
	add := func(col string, val any) {
		sets = append(sets, fmt.Sprintf("%s = $%d", col, i))
		args = append(args, val)
		i++
	}
	for k, col := range map[string]string{
		"name": "name", "account": "account_name", "stage": "stage", "owner": "owner",
		"currency": "currency", "description": "description", "source": "source", "notes": "notes",
		"amount_display": "amount_display",
	} {
		if v, ok := patch[k].(string); ok {
			add(col, v)
		}
	}
	if v, ok := patch["probability"].(float64); ok {
		add("probability", int(v))
	}
	if v, ok := patch["amount"].(float64); ok {
		add("amount", v)
	}
	if v, ok := patch["dms_linked"].(bool); ok {
		add("dms_linked", v)
	}
	// close_date was create-only: a deal's expected close could be set once and
	// never corrected, and the UI's edit form silently discarded it.
	if v, ok := patch["close_date"]; ok {
		add("close_date", parsePatchTime(v))
	}
	if attrs, ok := patchAttrs(patch); ok {
		add("attrs", encodeAttrs(attrs))
	}
	if len(sets) == 1 {
		return r.GetDeal(ctx, id)
	}
	args = append(args, id)
	_, err := r.db(ctx).Exec(ctx, `UPDATE crm_deals SET `+strings.Join(sets, ", ")+` WHERE id = $`+fmt.Sprint(i), args...)
	if err != nil {
		return models.Deal{}, err
	}
	return r.GetDeal(ctx, id)
}

func (r *Repository) SetDealStage(ctx context.Context, id, stage string) (models.Deal, error) {
	prob := stageDefaultProbability(stage)
	extra := ""
	if stage == models.DealStageWon {
		extra = ", won_at = NOW()"
	}
	if stage == models.DealStageLost {
		extra = ", lost_at = NOW()"
	}
	_, err := r.db(ctx).Exec(ctx, `
		UPDATE crm_deals SET stage = $2, probability = $3, updated_at = NOW()`+extra+` WHERE id = $1
	`, id, stage, prob)
	if err != nil {
		return models.Deal{}, err
	}
	return r.GetDeal(ctx, id)
}

func (r *Repository) PipelineBoard(ctx context.Context, owner string) ([]models.PipelineColumn, error) {
	labels := map[string]string{
		models.DealStageLead:        "Lead",
		models.DealStageQualified:   "Qualified",
		models.DealStageProposal:    "Proposal",
		models.DealStageNegotiation: "Negotiation",
		models.DealStageWon:         "Won",
	}
	var cols []models.PipelineColumn
	for _, stage := range []string{
		models.DealStageLead, models.DealStageQualified, models.DealStageProposal, models.DealStageNegotiation,
	} {
		opts := ListOpts{Limit: 100, Stage: stage, Owner: owner}
		deals, _, err := r.ListDeals(ctx, opts)
		if err != nil {
			return nil, err
		}
		var weighted float64
		for _, d := range deals {
			weighted += d.Amount * float64(d.Probability) / 100
		}
		cols = append(cols, models.PipelineColumn{
			Stage:       stage,
			Label:       labels[stage],
			Count:       len(deals),
			WeightedSum: weighted,
			Deals:       deals,
		})
	}
	return cols, nil
}

func (r *Repository) PipelineSummary(ctx context.Context) (map[string]any, error) {
	row := r.db(ctx).QueryRow(ctx, `
		SELECT
			COUNT(*)::int,
			COALESCE(SUM(amount * probability / 100.0), 0),
			COALESCE(AVG(amount), 0),
			COALESCE(AVG(EXTRACT(EPOCH FROM (COALESCE(won_at, NOW()) - created_at)) / 86400), 0)
		FROM crm_deals WHERE stage NOT IN ('lost')
	`)
	var count int
	var weighted, avgDeal, avgDays float64
	if err := row.Scan(&count, &weighted, &avgDeal, &avgDays); err != nil {
		return nil, err
	}
	var won, closed int
	_ = r.db(ctx).QueryRow(ctx, `SELECT COUNT(*) FILTER (WHERE stage = 'won'), COUNT(*) FILTER (WHERE stage IN ('won','lost')) FROM crm_deals`).Scan(&won, &closed)
	winRate := 0.0
	if closed > 0 {
		winRate = float64(won) / float64(closed) * 100
	}
	return map[string]any{
		"deal_count":        count,
		"pipeline_weighted": weighted,
		"avg_deal_size":     avgDeal,
		"velocity_days":     int(avgDays),
		"win_rate":          winRate,
	}, nil
}
