package register

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/Formulary-Labs/substrate/evidence"
)

// evalLog is the minimal EvaluationLog structure that ElevateFromEvaluationLog reads.
// It matches the JSON shape produced by assay's EvaluationLog output.
type evalLog struct {
	Evaluations []evalLogEntry `json:"evaluations"`
}

type evalLogEntry struct {
	Control struct {
		ReferenceID string `json:"reference_id"`
	} `json:"control"`
	Result         string `json:"result"` // "Passed" | "Failed" | "Needs Review"
	Message        string `json:"message"`
	AssessmentLogs []struct {
		ConfidenceLevel string `json:"confidence_level"` // "High" | "Medium" | "Low"
		Recommendation  string `json:"recommendation"`
	} `json:"assessment_logs"`
}

// ElevateFromEvaluationLog reads an assay EvaluationLog JSON at path and re-scores
// each risk in the register using three-signal elevation applied in priority order:
//
//  1. Evidence gap — risk.EvidenceRef is empty or a PENDING placeholder → always CRITICAL.
//  2. EvaluationLog failure — the risk's ControlID has a "Failed" or "Needs Review"
//     result in the log → likelihood elevated by confidence level.
//  3. Inherent baseline — no signal fires; existing likelihood/impact unchanged.
//
// Elevation table (mirrors psc-ms exactly):
//   - Failed + High confidence   → likelihood 4
//   - Failed + Medium            → likelihood 3
//   - Failed + Low               → likelihood 2; treatment TREAT
//   - Needs Review + High        → likelihood 3
//   - Needs Review + Medium/Low  → likelihood 2; treatment TOLERATE
//   - Evidence gap               → likelihood 5, impact 4 → always CRITICAL
//
// A Note is appended to each re-scored risk describing the signal that fired.
// Returns the list of RISK-NNN IDs that were re-scored.
func (r *Register) ElevateFromEvaluationLog(path string) ([]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading evaluation log %q: %w", path, err)
	}
	var log evalLog
	if err := json.Unmarshal(data, &log); err != nil {
		return nil, fmt.Errorf("parsing evaluation log %q: %w", path, err)
	}

	// Index evaluations by control reference_id; one control may have multiple entries.
	byControl := make(map[string][]evalLogEntry, len(log.Evaluations))
	for _, e := range log.Evaluations {
		id := e.Control.ReferenceID
		byControl[id] = append(byControl[id], e)
	}

	var elevated []string
	now := time.Now().UTC()

	for i := range r.Risks {
		risk := &r.Risks[i]

		var newLikelihood, newImpact int
		var note string

		// Signal 1: evidence gap — highest priority.
		if _, isPending := evidence.ClassifyLink(risk.EvidenceRef); isPending {
			newLikelihood, newImpact = 5, 4
			note = "Elevated to CRITICAL: EvidenceRef is empty or a PENDING placeholder (evidence_gap signal)."
		} else if risk.ControlID != "" {
			// Signal 2: EvaluationLog failure.
			for _, e := range byControl[risk.ControlID] {
				result := strings.ToLower(e.Result)
				if result != "failed" && result != "needs review" {
					continue
				}
				confidence := "low"
				recommendation := ""
				if len(e.AssessmentLogs) > 0 {
					confidence = strings.ToLower(e.AssessmentLogs[0].ConfidenceLevel)
					recommendation = e.AssessmentLogs[0].Recommendation
				}
				// Preserve existing impact if set; default to 3.
				impact := risk.Impact
				if impact == 0 {
					impact = 3
				}
				newImpact = impact

				if result == "failed" {
					switch confidence {
					case "high":
						newLikelihood = 4
					case "medium":
						newLikelihood = 3
					default:
						newLikelihood = 2
					}
					note = fmt.Sprintf("Elevated by EvaluationLog: result=Failed, confidence=%s, control=%s, treatment=TREAT.",
						confidence, risk.ControlID)
				} else {
					if confidence == "high" {
						newLikelihood = 3
					} else {
						newLikelihood = 2
					}
					note = fmt.Sprintf("Elevated by EvaluationLog: result=Needs Review, confidence=%s, control=%s, treatment=TOLERATE.",
						confidence, risk.ControlID)
				}
				if recommendation != "" {
					note += " " + recommendation
				}
				msg := e.Message
				if msg != "" {
					note += " " + msg
				}
				break
			}
		}

		if note == "" {
			continue
		}

		if newLikelihood > 0 {
			risk.Likelihood = newLikelihood
		}
		if newImpact > 0 {
			risk.Impact = newImpact
		}
		risk.Severity = computeSeverity(risk.Likelihood, risk.Impact)
		risk.Updated = now
		risk.Notes = append(risk.Notes, Note{
			Date: now.Format("2006-01-02"),
			Text: note,
		})
		elevated = append(elevated, risk.ID)
	}

	return elevated, nil
}
