package commerce

import (
	"testing"

	"github.com/iag/crm/backend/internal/usersclient"
)

func TestDeriveCustomerRefFromLegalName(t *testing.T) {
	ref := usersclient.DeriveCustomerRef(&usersclient.BillingIdentity{LegalName: "Matsuri Coffee Ltd"})
	if ref != "MATSURI-COFFEE-LTD" {
		t.Fatalf("unexpected ref: %s", ref)
	}
}
