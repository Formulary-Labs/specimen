package register_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/Formulary-Labs/specimen/register"
)

func TestAdd_createsStableID(t *testing.T) {
	reg := &register.Register{Program: "test"}

	id1 := reg.Add(register.Risk{
		Title:      "Risk one",
		Source:     register.SourceExplicit,
		Likelihood: 2, Impact: 2,
	})
	id2 := reg.Add(register.Risk{
		Title:      "Risk two",
		Source:     register.SourceExplicit,
		Likelihood: 1, Impact: 1,
	})

	if id1 != "RISK-001" {
		t.Errorf("first ID = %q, want RISK-001", id1)
	}
	if id2 != "RISK-002" {
		t.Errorf("second ID = %q, want RISK-002", id2)
	}
}

func TestAdd_computesSeverity(t *testing.T) {
	reg := &register.Register{Program: "test"}

	// 3x3 = 9 = critical
	id := reg.Add(register.Risk{Likelihood: 3, Impact: 3})
	for _, r := range reg.Risks {
		if r.ID == id && r.Severity != register.SeverityCritical {
			t.Errorf("3x3 severity = %q, want critical", r.Severity)
		}
	}

	// 1x1 = 1 = low
	id = reg.Add(register.Risk{Likelihood: 1, Impact: 1})
	for _, r := range reg.Risks {
		if r.ID == id && r.Severity != register.SeverityLow {
			t.Errorf("1x1 severity = %q, want low", r.Severity)
		}
	}
}

func TestFilter(t *testing.T) {
	reg := &register.Register{Program: "test"}
	reg.Add(register.Risk{Likelihood: 3, Impact: 3, Source: register.SourceExplicit}) // critical
	reg.Add(register.Risk{Likelihood: 1, Impact: 1, Source: register.SourceExplicit}) // low

	critical := reg.Filter("critical", "", "")
	if len(critical) != 1 {
		t.Errorf("expected 1 critical risk, got %d", len(critical))
	}
}

func TestSaveLoad(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "risks.json")

	reg := &register.Register{
		SchemaVersion: "1.0",
		Program:       "test",
	}
	reg.Add(register.Risk{Title: "Test risk", Source: register.SourceExplicit, Likelihood: 2, Impact: 2})

	if err := reg.Save(path); err != nil {
		t.Fatalf("Save error: %v", err)
	}

	loaded, err := register.Load(path, "test")
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	if len(loaded.Risks) != 1 {
		t.Errorf("expected 1 risk, got %d", len(loaded.Risks))
	}
	if loaded.Risks[0].Title != "Test risk" {
		t.Errorf("title = %q, want %q", loaded.Risks[0].Title, "Test risk")
	}
}

func TestIngestFeedForward(t *testing.T) {
	ff := []register.FeedForwardEntry{
		{
			FindingID:        "FF-001",
			Title:            "MFA not enforced",
			ControlID:        "A.8.5",
			Severity:         "high",
			CorrectiveAction: "Enable MFA for all admin accounts",
			Owner:            "security-team",
		},
	}
	data, _ := json.Marshal(ff)

	reg := &register.Register{Program: "test"}
	ids, err := reg.IngestFeedForward(data)
	if err != nil {
		t.Fatalf("IngestFeedForward error: %v", err)
	}
	if len(ids) != 1 {
		t.Errorf("expected 1 ingested risk, got %d", len(ids))
	}
	if reg.Risks[0].Source != register.SourceFeedForward {
		t.Errorf("source = %q, want feed_forward", reg.Risks[0].Source)
	}
	if reg.Risks[0].AuditFindingRef != "FF-001" {
		t.Errorf("audit_finding_ref = %q, want FF-001", reg.Risks[0].AuditFindingRef)
	}

	_ = os.Remove("risks.json") // clean up if accidentally written
}
