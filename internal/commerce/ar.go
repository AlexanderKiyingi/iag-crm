// Package commerce wires CRM revenue moments to finance AR.
package commerce

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/iag/crm/backend/internal/financeclient"
	"github.com/iag/crm/backend/internal/models"
	"github.com/iag/crm/backend/internal/store"
	"github.com/iag/crm/backend/internal/usersclient"
)

// BillingContext holds resolved customer billing keys.
type BillingContext struct {
	OrgID             string
	BillingIdentityID string
	CustomerRef       string
}

// ResolveBilling loads billing context for an account.
func ResolveBilling(
	ctx context.Context,
	repo *store.Repository,
	users *usersclient.Client,
	accountID, accountName string,
) (BillingContext, error) {
	var bc BillingContext
	if accountID != "" {
		acct, err := repo.GetAccount(ctx, accountID)
		if err == nil {
			bc.OrgID = acct.BillingOrgID
			bc.BillingIdentityID = acct.BillingIdentityID
			if acct.FinanceCustomerRef != "" {
				bc.CustomerRef = acct.FinanceCustomerRef
				return bc, nil
			}
			if bc.OrgID != "" && bc.BillingIdentityID != "" && users != nil && users.Enabled() {
				bi, err := users.GetBillingIdentity(ctx, bc.OrgID, bc.BillingIdentityID)
				if err == nil {
					bc.CustomerRef = usersclient.DeriveCustomerRef(bi)
					return bc, nil
				}
			}
			if acct.Name != "" {
				bc.CustomerRef = usersclient.DeriveCustomerRef(&usersclient.BillingIdentity{LegalName: acct.Name})
				return bc, nil
			}
		}
	}
	name := strings.TrimSpace(accountName)
	if name != "" {
		bc.CustomerRef = usersclient.DeriveCustomerRef(&usersclient.BillingIdentity{LegalName: name})
	}
	return bc, nil
}

// BookDealAR creates finance AR for a won deal when finance is configured.
func BookDealAR(
	ctx context.Context,
	finance *financeclient.Client,
	repo *store.Repository,
	users *usersclient.Client,
	deal models.Deal,
) (string, error) {
	if finance == nil || !finance.Enabled() {
		return "", nil
	}
	if deal.FinanceARRef != "" {
		return deal.FinanceARRef, nil
	}
	bc, err := ResolveBilling(ctx, repo, users, deal.AccountID, deal.Account)
	if err != nil {
		return "", err
	}
	if bc.CustomerRef == "" {
		return "", fmt.Errorf("no customer ref for deal %s", deal.ID)
	}
	amount := strconv.FormatFloat(deal.Amount, 'f', 2, 64)
	currency := deal.Currency
	if currency == "" {
		currency = "USD"
	}
	docRef := "CRM-DEAL-" + deal.ID
	ref, err := finance.CreateARItem(ctx, financeclient.CreateARInput{
		OrgID:             bc.OrgID,
		BillingIdentityID: bc.BillingIdentityID,
		CustomerRef:       bc.CustomerRef,
		DocumentRef:       docRef,
		Description:       "Won deal: " + deal.Name,
		Amount:            amount,
		Currency:          currency,
	})
	if err != nil {
		return "", err
	}
	_ = repo.SetDealFinanceARRef(ctx, deal.ID, ref)
	return ref, nil
}

// BookQuoteAR creates finance AR for a signed quote.
func BookQuoteAR(
	ctx context.Context,
	finance *financeclient.Client,
	repo *store.Repository,
	users *usersclient.Client,
	quote models.Quote,
) (string, error) {
	if finance == nil || !finance.Enabled() {
		return "", nil
	}
	if quote.FinanceARRef != "" {
		return quote.FinanceARRef, nil
	}
	bc, err := ResolveBilling(ctx, repo, users, quote.AccountID, quote.Account)
	if err != nil {
		return "", err
	}
	if bc.CustomerRef == "" {
		return "", fmt.Errorf("no customer ref for quote %s", quote.ID)
	}
	amount := strconv.FormatFloat(quote.Total, 'f', 2, 64)
	currency := quote.Currency
	if currency == "" {
		currency = "USD"
	}
	docRef := "CRM-QTE-" + quote.Ref
	if docRef == "CRM-QTE-" {
		docRef = "CRM-QTE-" + quote.ID
	}
	ref, err := finance.CreateARItem(ctx, financeclient.CreateARInput{
		OrgID:             bc.OrgID,
		BillingIdentityID: bc.BillingIdentityID,
		CustomerRef:       bc.CustomerRef,
		DocumentRef:       docRef,
		Description:       "Signed quote: " + quote.Ref,
		Amount:            amount,
		Currency:          currency,
	})
	if err != nil {
		return "", err
	}
	_ = repo.SetQuoteFinanceARRef(ctx, quote.ID, ref)
	return ref, nil
}
