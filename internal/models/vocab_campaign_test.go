package models

import "testing"

func TestCampaignStatusMachine(t *testing.T) {
	if got := NormalizeCampaignStatus("Planned"); got != CampaignStatusDraft {
		t.Fatalf("planned → draft, got %s", got)
	}
	if !CanTransitionCampaign(CampaignStatusDraft, CampaignStatusPendingCEO) {
		t.Fatal("draft should move to pending_ceo")
	}
	if CanTransitionCampaign(CampaignStatusDraft, CampaignStatusApproved) {
		t.Fatal("draft must not skip CEO queue")
	}
	if !CanTransitionCampaign(CampaignStatusPendingCEO, CampaignStatusApproved) {
		t.Fatal("pending_ceo should move to approved")
	}
	if CanTransitionCampaign(CampaignStatusDraft, CampaignStatusLive) {
		t.Fatal("draft must not jump to live")
	}
	if !CanTransitionCampaign(CampaignStatusApproved, CampaignStatusLive) {
		t.Fatal("approved should go live")
	}
}
