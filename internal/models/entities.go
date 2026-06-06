package models

import (
	"encoding/json"
	"time"
)

const (
	DealStageLead         = "lead"
	DealStageQualified    = "qualified"
	DealStageProposal     = "proposal"
	DealStageNegotiation  = "negotiation"
	DealStageWon          = "won"
	DealStageLost         = "lost"

	AccountStatusActive  = "active"
	AccountStatusNurture = "nurture"
	AccountStatusRisk    = "risk"
)

var DealStages = []string{
	DealStageLead,
	DealStageQualified,
	DealStageProposal,
	DealStageNegotiation,
	DealStageWon,
	DealStageLost,
}

type Account struct {
	ID                 string    `json:"id"`
	Name               string    `json:"name"`
	Type               string    `json:"type"`
	Country            string    `json:"country"`
	Segment            string    `json:"segment"`
	Owner              string    `json:"owner"`
	Value              string    `json:"value"`
	Health             int       `json:"health"`
	Status             string    `json:"status"`
	Bridged            bool      `json:"bridged"`
	LastTouch          string    `json:"last_touch"`
	Email              string    `json:"email,omitempty"`
	Phone              string    `json:"phone,omitempty"`
	Address            string    `json:"address,omitempty"`
	DmsRef             string    `json:"dms_ref,omitempty"`
	BillingOrgID       string    `json:"billing_org_id,omitempty"`
	BillingIdentityID  string    `json:"billing_identity_id,omitempty"`
	FinanceCustomerRef string    `json:"finance_customer_ref,omitempty"`
	LastTouchAt        time.Time `json:"last_touch_at"`
	CreatedAt          time.Time `json:"created_at"`
	UpdatedAt          time.Time `json:"updated_at"`
}

type Contact struct {
	ID          string    `json:"id"`
	AccountID   string    `json:"account_id,omitempty"`
	Account     string    `json:"account"`
	Name        string    `json:"name"`
	Title       string    `json:"title"`
	Email       string    `json:"email"`
	Phone       string    `json:"phone"`
	Owner       string    `json:"owner"`
	BuyerRole   string    `json:"buyer_role,omitempty"`
	Primary     bool      `json:"primary"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type Lead struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Company   string    `json:"company"`
	Email     string    `json:"email"`
	Phone     string    `json:"phone"`
	Source    string    `json:"source"`
	Segment   string    `json:"segment"`
	Region    string    `json:"region,omitempty"`
	Score     int       `json:"score"`
	Status    string    `json:"status"`
	Owner     string    `json:"owner"`
	Notes     string    `json:"notes,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type Deal struct {
	ID            string     `json:"id"`
	Name          string     `json:"name"`
	AccountID     string     `json:"account_id,omitempty"`
	Account       string     `json:"account"`
	Stage         string     `json:"stage"`
	Probability   int        `json:"probability"`
	Owner         string     `json:"owner"`
	Currency      string     `json:"currency"`
	Amount        float64    `json:"amount"`
	AmountDisplay string     `json:"amount_display"`
	Description   string     `json:"description,omitempty"`
	Source        string     `json:"source,omitempty"`
	DmsLinked     bool       `json:"dms_linked"`
	CloseDate     *time.Time `json:"close_date,omitempty"`
	Notes         string     `json:"notes,omitempty"`
	FinanceARRef  string     `json:"finance_ar_ref,omitempty"`
	AgeDays       int        `json:"age_days,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}

type QuoteLineItem struct {
	LotRef   string  `json:"lot_ref"`
	Quantity int     `json:"quantity"`
	Unit     string  `json:"unit,omitempty"`
	Price    float64 `json:"price,omitempty"`
}

type Quote struct {
	ID           string          `json:"id"`
	Ref          string          `json:"ref"`
	AccountID    string          `json:"account_id,omitempty"`
	Account      string          `json:"account"`
	DealID       string          `json:"deal_id,omitempty"`
	Template     string          `json:"template"`
	Currency     string          `json:"currency"`
	Incoterms    string          `json:"incoterms"`
	PaymentTerms string          `json:"payment_terms"`
	ValidUntil   *time.Time      `json:"valid_until,omitempty"`
	Total        float64         `json:"total"`
	Status       string          `json:"status"`
	Version      int             `json:"version"`
	Owner         string          `json:"owner"`
	LineItems     []QuoteLineItem `json:"line_items"`
	FinanceARRef  string          `json:"finance_ar_ref,omitempty"`
	ContractRef   string          `json:"contract_ref,omitempty"`
	CreatedAt     time.Time       `json:"created_at"`
	UpdatedAt     time.Time       `json:"updated_at"`
}

type Activity struct {
	ID           string    `json:"id"`
	Type         string    `json:"type"`
	Subject      string    `json:"subject"`
	Body         string    `json:"body,omitempty"`
	AccountID    string    `json:"account_id,omitempty"`
	Account      string    `json:"account,omitempty"`
	ContactID    string    `json:"contact_id,omitempty"`
	DealID       string    `json:"deal_id,omitempty"`
	OutletRef    string    `json:"outlet_ref,omitempty"`
	Owner        string    `json:"owner"`
	OccurredAt   time.Time `json:"occurred_at"`
	CreatedAt    time.Time `json:"created_at"`
}

type Ticket struct {
	ID          string     `json:"id"`
	AccountID   string     `json:"account_id,omitempty"`
	Account     string     `json:"account"`
	ContactID   string     `json:"contact_id,omitempty"`
	OutletRef   string     `json:"outlet_ref,omitempty"`
	DealID      string     `json:"deal_id,omitempty"`
	Subject     string     `json:"subject"`
	Type        string     `json:"type"`
	Priority    string     `json:"priority"`
	Channel     string     `json:"channel,omitempty"`
	Status      string     `json:"status"`
	Owner       string     `json:"owner"`
	Description string     `json:"description,omitempty"`
	DmsClaimRef string     `json:"dms_claim_ref,omitempty"`
	SlaDueAt    *time.Time `json:"sla_due_at,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

type Campaign struct {
	ID        string     `json:"id"`
	Name      string     `json:"name"`
	Type      string     `json:"type"`
	Audience  string     `json:"audience"`
	StartsOn  *time.Time `json:"starts_on,omitempty"`
	EndsOn    *time.Time `json:"ends_on,omitempty"`
	BudgetUSD float64    `json:"budget_usd,omitempty"`
	Owner     string     `json:"owner"`
	Goal      string     `json:"goal,omitempty"`
	Status    string     `json:"status"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
}

type Notification struct {
	Kind  string `json:"kind"`
	Icon  string `json:"icon"`
	Title string `json:"title"`
	Body  string `json:"body"`
	Time  string `json:"time"`
	Go    string `json:"go"`
}

type PipelineColumn struct {
	Stage       string  `json:"stage"`
	Label       string  `json:"label"`
	Count       int     `json:"count"`
	WeightedSum float64 `json:"weighted_sum"`
	Deals       []Deal  `json:"deals"`
}

type RangeMetrics struct {
	Label          string `json:"label"`
	Pipeline       string `json:"pipeline"`
	PipelineDelta  string `json:"pipeline_delta"`
	Outlets        string `json:"outlets"`
	WinRate        string `json:"win_rate"`
	Cycle          string `json:"cycle"`
	NRR            string `json:"nrr"`
	Series         []int  `json:"series"`
	XLabels        []string `json:"x_labels"`
}

type Bootstrap struct {
	Service     string            `json:"service"`
	Role        string            `json:"role"`
	RoleLabel   string            `json:"role_label"`
	Pages       []string          `json:"pages"`
	Modals      []string          `json:"modals"`
	PageTitles  map[string]string `json:"page_titles"`
	SyncStatus  string            `json:"sync_status"`
}

type ListMeta struct {
	Total  int `json:"total"`
	Limit  int `json:"limit"`
	Offset int `json:"offset"`
}

type Paginated[T any] struct {
	Data []T      `json:"data"`
	Meta ListMeta `json:"meta"`
}

type AuditEntry struct {
	ID       string    `json:"id"`
	LoggedAt time.Time `json:"logged_at"`
	UserName string    `json:"user_name"`
	Action   string    `json:"action"`
	Detail   string    `json:"detail"`
}

type PermissionCheckInput struct {
	Keys []string `json:"keys"`
}

type PermissionCheckResult struct {
	Allowed map[string]bool `json:"allowed"`
}

type PermissionContext struct {
	Role           string   `json:"role"`
	Email          string   `json:"email"`
	Name           string   `json:"name"`
	Permissions    []string `json:"permissions"`
	CanMutate      bool     `json:"canMutate"`
	CanManageAdmin bool     `json:"canManageAdmin"`
	IsStaff        bool     `json:"isStaff"`
}

func RawJSON(v any) json.RawMessage {
	b, _ := json.Marshal(v)
	return b
}
