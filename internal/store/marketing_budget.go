package store

import "math"

// budgetBurnPct returns percent of planned channel budget consumed by live spend.
func budgetBurnPct(planned, spent float64) float64 {
	if planned <= 0 {
		return 0
	}
	pct := spent / planned * 100
	if pct > 100 {
		return 100
	}
	return math.Round(pct*10) / 10
}
