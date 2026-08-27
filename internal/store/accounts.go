package store

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/iag/crm/backend/internal/models"
)

// resolveAccountID looks up an account id by exact name. It returns ("", nil)
// when no account matches, so the caller stores a null FK; a real query error is
// propagated instead of being silently swallowed into a dangling reference.
func (r *Repository) resolveAccountID(ctx context.Context, name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", nil
	}
	var id string
	err := r.db(ctx).QueryRow(ctx, `SELECT id FROM crm_accounts WHERE name = $1 LIMIT 1`, name).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("resolve account by name: %w", err)
	}
	return id, nil
}

type Repository struct {
	pool     *pgxpool.Pool
	tokenKey []byte
}

func New(pool *pgxpool.Pool, opts ...Option) *Repository {
	r := &Repository{pool: pool}
	for _, opt := range opts {
		opt(r)
	}
	return r
}

func (r *Repository) Ping(ctx context.Context) error {
	return r.pool.Ping(ctx)
}

func (r *Repository) IsEmpty(ctx context.Context) (bool, error) {
	var n int
	err := r.db(ctx).QueryRow(ctx, `SELECT COUNT(*)::int FROM crm_accounts`).Scan(&n)
	return n == 0, err
}

// NewID mints a primary key. CRM used to draw prefixed, sequential ids from
// crm_id_counters (ACC-0500, LEAD-0200); ids are uuid across the platform now,
// so the counter table and its prefixes are gone.
func (r *Repository) NewID() string {
	return uuid.NewString()
}

type ListOpts struct {
	Limit  int
	Offset int
	// Owner is the caller's own filter, straight from ?owner=. It narrows a
	// result set; it must never be able to widen one.
	Owner  string
	Stage  string
	Status string
	Search string
	Type   string

	// ScopeOwner is the ENFORCED visibility boundary, set from the caller's
	// identity and never from request input.
	//
	// It is deliberately a separate field from Owner. Scoping used to work by
	// defaulting Owner to the caller's email when the query string had not
	// supplied one — which meant a sales rep could pass ?owner=someone@else and
	// read another rep's records, because the parameter the scope relied on was
	// the same one the caller controlled. Keeping them apart means the two are
	// ANDed: a rep may filter within their own records, and cannot escape them.
	ScopeOwner string
}

// applyScope appends the enforced owner predicate to a query's WHERE clause.
//
// Every list over an owner-bearing sales table calls this. It is a no-op when
// no scope is set (managers, superusers, service callers), so the same query
// serves both cases without a second code path to keep in sync.
func applyScope(opts ListOpts, col string, where []string, args []any, i *int) ([]string, []any) {
	if opts.ScopeOwner == "" {
		return where, args
	}
	where = append(where, fmt.Sprintf("%s = $%d", col, *i))
	args = append(args, opts.ScopeOwner)
	*i++
	return where, args
}

func clampLimit(limit int) int {
	if limit <= 0 {
		return 50
	}
	if limit > 200 {
		return 200
	}
	return limit
}

func relativeTouch(t time.Time) string {
	diff := time.Since(t)
	switch {
	case diff < time.Minute:
		return "just now"
	case diff < time.Hour:
		return fmt.Sprintf("%dm ago", int(diff.Minutes()))
	case diff < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(diff.Hours()))
	default:
		return fmt.Sprintf("%dd ago", int(diff.Hours()/24))
	}
}

func scanAccount(row pgx.Row) (models.Account, error) {
	var a models.Account
	var amount *float64
	var currency *string
	var billingOrg, billingID, financeRef *string
	// dms_ref, email, phone and address are all nullable, and CreateAccount
	// writes NULL into dms_ref for any account with no DMS reference
	// (nullStr turns "" into nil). Scanning NULL into a plain string errors, and
	// a scan error on one row fails the whole query — so a single account
	// created without a DMS ref would have taken down GET /accounts for
	// everyone, not just its own record. The billing columns beside them were
	// already read this way; these four were not.
	var dmsRef, email, phone, address *string
	err := row.Scan(
		&a.ID, &a.Name, &a.Type, &a.Country, &a.Segment, &a.Owner,
		&a.Value, &amount, &currency, &a.Health, &a.Status, &a.Bridged,
		&dmsRef, &email, &phone, &address,
		&billingOrg, &billingID, &financeRef,
		&a.LastTouchAt, &a.CreatedAt, &a.UpdatedAt,
	)
	if dmsRef != nil {
		a.DmsRef = *dmsRef
	}
	if email != nil {
		a.Email = *email
	}
	if phone != nil {
		a.Phone = *phone
	}
	if address != nil {
		a.Address = *address
	}
	if billingOrg != nil {
		a.BillingOrgID = *billingOrg
	}
	if billingID != nil {
		a.BillingIdentityID = *billingID
	}
	if financeRef != nil {
		a.FinanceCustomerRef = *financeRef
	}
	if err != nil {
		return a, err
	}
	a.LastTouch = relativeTouch(a.LastTouchAt)
	return a, nil
}

func (r *Repository) ListAccounts(ctx context.Context, opts ListOpts) ([]models.Account, int, error) {
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
	if opts.Search != "" {
		where = append(where, fmt.Sprintf("(name ILIKE $%d OR segment ILIKE $%d)", i, i))
		args = append(args, "%"+opts.Search+"%")
		i++
	}
	whereSQL := strings.Join(where, " AND ")

	var total int
	if err := r.db(ctx).QueryRow(ctx, "SELECT COUNT(*)::int FROM crm_accounts WHERE "+whereSQL, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	args = append(args, opts.Limit, opts.Offset)
	rows, err := r.db(ctx).Query(ctx, `
		SELECT id, name, account_type, country, segment, owner, value_display,
		       value_amount, value_currency, health_score, status, dms_bridged,
		       COALESCE(dms_ref, ''), COALESCE(email, ''), COALESCE(phone, ''), COALESCE(address, ''),
		       billing_org_id, billing_identity_id, finance_customer_ref,
		       last_touch_at, created_at, updated_at
		FROM crm_accounts
		WHERE `+whereSQL+`
		ORDER BY last_touch_at DESC
		LIMIT $`+fmt.Sprint(i)+` OFFSET $`+fmt.Sprint(i+1), args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var out []models.Account
	for rows.Next() {
		a, err := scanAccount(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, a)
	}
	return out, total, rows.Err()
}

func (r *Repository) GetAccount(ctx context.Context, id string) (models.Account, error) {
	row := r.db(ctx).QueryRow(ctx, `
		SELECT id, name, account_type, country, segment, owner, value_display,
		       value_amount, value_currency, health_score, status, dms_bridged,
		       COALESCE(dms_ref, ''), COALESCE(email, ''), COALESCE(phone, ''), COALESCE(address, ''),
		       billing_org_id, billing_identity_id, finance_customer_ref,
		       last_touch_at, created_at, updated_at
		FROM crm_accounts WHERE id = $1
	`, id)
	return scanAccount(row)
}

type AccountInput struct {
	Name               string  `json:"name"`
	Type               string  `json:"type"`
	Country            string  `json:"country"`
	Segment            string  `json:"segment"`
	Owner              string  `json:"owner"`
	Value              string  `json:"value"`
	Health             int     `json:"health"`
	Status             string  `json:"status"`
	Bridged            bool    `json:"bridged"`
	Email              string  `json:"email"`
	Phone              string  `json:"phone"`
	Address            string  `json:"address"`
	DmsRef             string  `json:"dms_ref"`
	BillingOrgID       string  `json:"billing_org_id"`
	BillingIdentityID  string  `json:"billing_identity_id"`
	FinanceCustomerRef string  `json:"finance_customer_ref"`
	Amount             float64 `json:"amount"`
	Currency           string  `json:"currency"`
}

func (r *Repository) CreateAccount(ctx context.Context, in AccountInput) (models.Account, error) {
	id := r.NewID()
	now := time.Now().UTC()
	if in.Status == "" {
		in.Status = models.AccountStatusActive
	}
	if in.Type == "" {
		in.Type = "Export"
	}
	_, err := r.db(ctx).Exec(ctx, `
		INSERT INTO crm_accounts (
			id, name, account_type, country, segment, owner, value_display,
			value_amount, value_currency, health_score, status, dms_bridged,
			dms_ref, email, phone, address, billing_org_id, billing_identity_id, finance_customer_ref,
			last_touch_at, created_at, updated_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$20,$20)
	`, id, in.Name, in.Type, in.Country, in.Segment, in.Owner, in.Value,
		nullFloat(in.Amount), nullStr(in.Currency), in.Health, in.Status, in.Bridged,
		nullStr(in.DmsRef), in.Email, in.Phone, in.Address,
		nullStr(in.BillingOrgID), nullStr(in.BillingIdentityID), nullStr(in.FinanceCustomerRef), now)
	if err != nil {
		return models.Account{}, err
	}
	return r.GetAccount(ctx, id)
}

func (r *Repository) PatchAccount(ctx context.Context, id string, patch map[string]any) (models.Account, error) {
	sets := []string{"updated_at = NOW()"}
	args := []any{}
	i := 1
	add := func(col string, val any) {
		sets = append(sets, fmt.Sprintf("%s = $%d", col, i))
		args = append(args, val)
		i++
	}
	if v, ok := patch["name"].(string); ok {
		add("name", v)
	}
	if v, ok := patch["type"].(string); ok {
		add("account_type", v)
	}
	if v, ok := patch["country"].(string); ok {
		add("country", v)
	}
	if v, ok := patch["segment"].(string); ok {
		add("segment", v)
	}
	if v, ok := patch["owner"].(string); ok {
		add("owner", v)
	}
	if v, ok := patch["value"].(string); ok {
		add("value_display", v)
	}
	if v, ok := patch["health"].(float64); ok {
		add("health_score", int(v))
	}
	if v, ok := patch["status"].(string); ok {
		add("status", v)
	}
	if v, ok := patch["bridged"].(bool); ok {
		add("dms_bridged", v)
	}
	if v, ok := patch["billing_org_id"].(string); ok {
		add("billing_org_id", v)
	}
	if v, ok := patch["billing_identity_id"].(string); ok {
		add("billing_identity_id", v)
	}
	if v, ok := patch["finance_customer_ref"].(string); ok {
		add("finance_customer_ref", v)
	}
	if len(sets) == 1 {
		return r.GetAccount(ctx, id)
	}
	args = append(args, id)
	_, err := r.db(ctx).Exec(ctx, `UPDATE crm_accounts SET `+strings.Join(sets, ", ")+` WHERE id = $`+fmt.Sprint(i), args...)
	if err != nil {
		return models.Account{}, err
	}
	return r.GetAccount(ctx, id)
}

func (r *Repository) DeleteAccount(ctx context.Context, id string) error {
	_, err := r.db(ctx).Exec(ctx, `DELETE FROM crm_accounts WHERE id = $1`, id)
	return err
}

func scanContact(row pgx.Row) (models.Contact, error) {
	var c models.Contact
	var accountID *string
	err := row.Scan(
		&c.ID, &accountID, &c.Account, &c.Name, &c.Title, &c.Email, &c.Phone,
		&c.BuyerRole, &c.Owner, &c.Primary, &c.CreatedAt, &c.UpdatedAt,
	)
	if err != nil {
		return c, err
	}
	if accountID != nil {
		c.AccountID = *accountID
	}
	return c, nil
}

func (r *Repository) ListContacts(ctx context.Context, opts ListOpts) ([]models.Contact, int, error) {
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
	if opts.Search != "" {
		where = append(where, fmt.Sprintf("(name ILIKE $%d OR email ILIKE $%d OR account_name ILIKE $%d)", i, i, i))
		args = append(args, "%"+opts.Search+"%")
		i++
	}
	whereSQL := strings.Join(where, " AND ")
	var total int
	if err := r.db(ctx).QueryRow(ctx, "SELECT COUNT(*)::int FROM crm_contacts WHERE "+whereSQL, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	args = append(args, opts.Limit, opts.Offset)
	rows, err := r.db(ctx).Query(ctx, `
		SELECT id, account_id, account_name, name, title, email, phone, buyer_role, owner, is_primary, created_at, updated_at
		FROM crm_contacts WHERE `+whereSQL+` ORDER BY name LIMIT $`+fmt.Sprint(i)+` OFFSET $`+fmt.Sprint(i+1), args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var out []models.Contact
	for rows.Next() {
		c, err := scanContact(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, c)
	}
	return out, total, rows.Err()
}

func (r *Repository) GetContact(ctx context.Context, id string) (models.Contact, error) {
	row := r.db(ctx).QueryRow(ctx, `
		SELECT id, account_id, account_name, name, title, email, phone, buyer_role, owner, is_primary, created_at, updated_at
		FROM crm_contacts WHERE id = $1
	`, id)
	return scanContact(row)
}

type ContactInput struct {
	Name      string `json:"name"`
	Title     string `json:"title"`
	Account   string `json:"account"`
	AccountID string `json:"account_id"`
	Email     string `json:"email"`
	Phone     string `json:"phone"`
	Owner     string `json:"owner"`
	BuyerRole string `json:"buyer_role"`
	Primary   bool   `json:"primary"`
}

func (r *Repository) CreateContact(ctx context.Context, in ContactInput) (models.Contact, error) {
	id := r.NewID()
	accountID := in.AccountID
	accountName := in.Account
	if accountID == "" && accountName != "" {
		aid, err := r.resolveAccountID(ctx, accountName)
		if err != nil {
			return models.Contact{}, err
		}
		accountID = aid
	}
	now := time.Now().UTC()
	_, err := r.db(ctx).Exec(ctx, `
		INSERT INTO crm_contacts (id, account_id, account_name, name, title, email, phone, buyer_role, owner, is_primary, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$11)
	`, id, nullStr(accountID), accountName, in.Name, in.Title, in.Email, in.Phone, in.BuyerRole, in.Owner, in.Primary, now)
	if err != nil {
		return models.Contact{}, err
	}
	return r.GetContact(ctx, id)
}

func (r *Repository) DeleteContact(ctx context.Context, id string) error {
	tag, err := r.db(ctx).Exec(ctx, `DELETE FROM crm_contacts WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

func (r *Repository) PatchContact(ctx context.Context, id string, patch map[string]any) (models.Contact, error) {
	sets := []string{}
	args := []any{id}
	i := 2
	for _, field := range []struct{ key, col string }{
		{"name", "name"}, {"title", "title"}, {"email", "email"}, {"phone", "phone"},
		{"owner", "owner"}, {"buyer_role", "buyer_role"}, {"account_name", "account_name"},
	} {
		if v, ok := patch[field.key]; ok {
			sets = append(sets, fmt.Sprintf("%s = $%d", field.col, i))
			args = append(args, v)
			i++
		}
	}
	if len(sets) == 0 {
		return r.GetContact(ctx, id)
	}
	sets = append(sets, "updated_at = NOW()")
	q := fmt.Sprintf("UPDATE crm_contacts SET %s WHERE id = $1", strings.Join(sets, ", "))
	tag, err := r.db(ctx).Exec(ctx, q, args...)
	if err != nil {
		return models.Contact{}, err
	}
	if tag.RowsAffected() == 0 {
		return models.Contact{}, pgx.ErrNoRows
	}
	return r.GetContact(ctx, id)
}

func nullStr(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func nullFloat(f float64) any {
	if f == 0 {
		return nil
	}
	return f
}
