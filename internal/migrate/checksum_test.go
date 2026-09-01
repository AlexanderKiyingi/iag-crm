package migrate

import (
	"crypto/sha256"
	"encoding/hex"
	"testing"
	"testing/fstest"
)

func sum(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}

// A ledger stamped from a Windows checkout — before .gitattributes pinned *.sql
// to eol=lf — holds the sha256 of a CRLF file. The Linux container hashes LF and
// must not read that as an edited migration.
func TestChecksumAcceptsEitherLineEnding(t *testing.T) {
	const lf = "CREATE TABLE IF NOT EXISTS t (id UUID);\nSELECT 1;\n"
	crlf := "CREATE TABLE IF NOT EXISTS t (id UUID);\r\nSELECT 1;\r\n"

	for name, body := range map[string]string{"lf": lf, "crlf": crlf} {
		t.Run(name, func(t *testing.T) {
			migs, err := load(fstest.MapFS{"0001_x.sql": {Data: []byte(body)}})
			if err != nil {
				t.Fatalf("load: %v", err)
			}
			m := migs[0]
			if m.Checksum != sum(body) {
				t.Fatalf("Checksum must hash the file as shipped: got %s", m.Checksum)
			}
			if !m.matches(sum(lf)) {
				t.Error("an LF-stamped ledger row must be recognised")
			}
			if !m.matches(sum(crlf)) {
				t.Error("a CRLF-stamped ledger row must be recognised")
			}
			if m.matches(sum(lf + "DROP TABLE t;\n")) {
				t.Error("a genuinely edited body must still mismatch")
			}
		})
	}
}
