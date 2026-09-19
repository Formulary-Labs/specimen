package register_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/Formulary-Labs/specimen/register"
)

// evalLogFixture builds minimal EvaluationLog JSON for test input.
func evalLogFixture(entries []map[string]any) []byte {
	data, _ := json.Marshal(map[string]any{"evaluations": entries})
	return data
}

func TestElevateFromEvaluationLog_evidenceGap(t *testing.T) {
	reg := &register.Register{Program: "test"}
	reg.Add(register.Risk{
		Title:       "No MFA",
		Source:      register.SourceExplicit,
		ControlID:   "A.8.5",
		EvidenceRef: "", // empty → evidence gap
		Likelihood:  1, Impact: 1,
	})

	dir := t.TempDir()
	logPath := filepath.Join(dir, "eval.json")
	// Empty evaluations — signal fires from evidence gap alone.
	os.WriteFile(logPath, evalLogFixture(nil), 0o644) //nolint:errcheck

	elevated, err := reg.ElevateFromEvaluationLog(logPath)
	if err != nil {
		t.Fatalf("ElevateFromEvaluationLog error: %v", err)
	}
	if len(elevated) != 1 {
		t.Fatalf("expected 1 elevated risk, got %d", len(elevated))
	}
	r := reg.Risks[0]
	if r.Severity != register.SeverityCritical {
		t.Errorf("severity = %q, want critical", r.Severity)
	}
	if r.Likelihood != 5 {
		t.Errorf("likelihood = %d, want 5", r.Likelihood)
	}
	if r.Impact != 4 {
		t.Errorf("impact = %d, want 4", r.Impact)
	}
	if len(r.Notes) == 0 {
		t.Error("expected a note to be appended")
	}
}

func TestElevateFromEvaluationLog_pendingRef(t *testing.T) {
	reg := &register.Register{Program: "test"}
	reg.Add(register.Risk{
		Title:       "Stale evidence",
		Source:      register.SourceExplicit,
		ControlID:   "A.5.1",
		EvidenceRef: "PENDING — not yet collected",
		Likelihood:  1, Impact: 1,
	})

	dir := t.TempDir()
	logPath := filepath.Join(dir, "eval.json")
	os.WriteFile(logPath, evalLogFixture(nil), 0o644) //nolint:errcheck

	elevated, _ := reg.ElevateFromEvaluationLog(logPath)
	if len(elevated) != 1 {
		t.Fatalf("expected 1 elevated risk, got %d", len(elevated))
	}
	if reg.Risks[0].Severity != register.SeverityCritical {
		t.Errorf("PENDING ref should elevate to critical, got %q", reg.Risks[0].Severity)
	}
}

func TestElevateFromEvaluationLog_failedHighConfidence(t *testing.T) {
	reg := &register.Register{Program: "test"}
	reg.Add(register.Risk{
		Title:       "Access control gap",
		Source:      register.SourceExplicit,
		ControlID:   "A.9.1",
		EvidenceRef: "https://github.com/org/repo/issues/42",
		Likelihood:  1, Impact: 2,
	})

	entry := map[string]any{
		"control": map[string]any{"reference_id": "A.9.1"},
		"result":  "Failed",
		"message": "Control not implemented",
		"assessment_logs": []map[string]any{
			{"confidence_level": "High", "recommendation": "Implement access controls immediately."},
		},
	}
	dir := t.TempDir()
	logPath := filepath.Join(dir, "eval.json")
	os.WriteFile(logPath, evalLogFixture([]map[string]any{entry}), 0o644) //nolint:errcheck

	elevated, err := reg.ElevateFromEvaluationLog(logPath)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if len(elevated) != 1 {
		t.Fatalf("expected 1 elevated, got %d", len(elevated))
	}
	if reg.Risks[0].Likelihood != 4 {
		t.Errorf("Failed+High should set likelihood=4, got %d", reg.Risks[0].Likelihood)
	}
}

func TestElevateFromEvaluationLog_needsReviewMedium(t *testing.T) {
	reg := &register.Register{Program: "test"}
	reg.Add(register.Risk{
		Title:       "Patch gap",
		Source:      register.SourceExplicit,
		ControlID:   "A.12.1",
		EvidenceRef: "https://jira.example.com/browse/SEC-1",
		Likelihood:  1, Impact: 2,
	})

	entry := map[string]any{
		"control": map[string]any{"reference_id": "A.12.1"},
		"result":  "Needs Review",
		"assessment_logs": []map[string]any{
			{"confidence_level": "Medium"},
		},
	}
	dir := t.TempDir()
	logPath := filepath.Join(dir, "eval.json")
	os.WriteFile(logPath, evalLogFixture([]map[string]any{entry}), 0o644) //nolint:errcheck

	reg.ElevateFromEvaluationLog(logPath) //nolint:errcheck
	if reg.Risks[0].Likelihood != 2 {
		t.Errorf("Needs Review+Medium should set likelihood=2, got %d", reg.Risks[0].Likelihood)
	}
}

func TestElevateFromEvaluationLog_passedNoChange(t *testing.T) {
	reg := &register.Register{Program: "test"}
	reg.Add(register.Risk{
		Title:       "Compliant control",
		Source:      register.SourceExplicit,
		ControlID:   "A.6.1",
		EvidenceRef: "https://github.com/org/repo/wiki",
		Likelihood:  1, Impact: 1,
	})

	entry := map[string]any{
		"control": map[string]any{"reference_id": "A.6.1"},
		"result":  "Passed",
	}
	dir := t.TempDir()
	logPath := filepath.Join(dir, "eval.json")
	os.WriteFile(logPath, evalLogFixture([]map[string]any{entry}), 0o644) //nolint:errcheck

	elevated, _ := reg.ElevateFromEvaluationLog(logPath)
	if len(elevated) != 0 {
		t.Errorf("Passed result should not elevate, got %d elevated", len(elevated))
	}
	if reg.Risks[0].Likelihood != 1 {
		t.Errorf("likelihood should be unchanged at 1, got %d", reg.Risks[0].Likelihood)
	}
}

func TestElevateFromEvaluationLog_missingFile(t *testing.T) {
	reg := &register.Register{Program: "test"}
	_, err := reg.ElevateFromEvaluationLog("/nonexistent/eval.json")
	if err == nil {
		t.Error("expected error for missing file")
	}
}
