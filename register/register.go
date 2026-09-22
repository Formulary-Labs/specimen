// Package register implements the risk register data model and operations
// for specimen. The register is a JSON file stored at
// data/[program]/risks.json.
//
// Risk IDs are stable: RISK-001, RISK-002, … — never reused after deletion.
// Severity is derived from a 3x3 likelihood/impact matrix.
package register

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Source identifies where the risk originated.
type Source string

//nolint:revive // Source constants are self-documenting string identifiers.
const (
	SourceCoverageGap Source = "coverage_gap"
	SourceEvidenceGap Source = "evidence_gap"
	SourceOwnerGap    Source = "owner_gap"
	SourceExplicit    Source = "explicit"
	SourceInferred    Source = "inferred"
	SourceFeedForward Source = "feed_forward"
	SourceCatalog     Source = "catalog" // imported from a gemara RiskCatalog
)

// Status is the current disposition of a risk.
type Status string

//nolint:revive // Status constants are self-documenting string identifiers.
const (
	StatusOpen      Status = "open"
	StatusAccepted  Status = "accepted"
	StatusMitigated Status = "mitigated"
	StatusClosed    Status = "closed"
)

// Severity is the risk severity derived from the 3x3 matrix.
type Severity string

//nolint:revive // Severity constants are self-documenting string identifiers.
const (
	SeverityCritical Severity = "critical"
	SeverityHigh     Severity = "high"
	SeverityMedium   Severity = "medium"
	SeverityLow      Severity = "low"
)

// Risk is a single risk entry.
type Risk struct {
	ID              string    `json:"id"`
	Title           string    `json:"title"`
	Description     string    `json:"description,omitempty"`
	Source          Source    `json:"source"`
	Likelihood      int       `json:"likelihood"` // 1-3
	Impact          int       `json:"impact"`     // 1-3
	Severity        Severity  `json:"severity"`
	Status          Status    `json:"status"`
	Owner           string    `json:"owner,omitempty"`
	ControlID       string    `json:"control_id,omitempty"`
	EvidenceRef     string    `json:"evidence_ref,omitempty"`
	RemediationPath string    `json:"remediation_path,omitempty"`
	AcceptRationale string    `json:"accept_rationale,omitempty"` // required when status=accepted
	Created         time.Time `json:"created"`
	Updated         time.Time `json:"updated"`
	Notes           []Note    `json:"notes,omitempty"`
	AuditFindingRef string    `json:"audit_finding_ref,omitempty"` // from feed-forward
	InferenceFlags  []string  `json:"inference_flags,omitempty"`
}

// Note is an append-only progress note.
type Note struct {
	Date   string `json:"date"`
	Text   string `json:"text"`
	Author string `json:"author,omitempty"`
}

// Register is the full risk register.
type Register struct {
	SchemaVersion string    `json:"schema_version"`
	Program       string    `json:"program"`
	LastUpdated   time.Time `json:"last_updated"`
	Risks         []Risk    `json:"risks"`
}

// Load reads a risk register from disk. Returns an empty register if the
// file does not exist.
func Load(path, program string) (*Register, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return &Register{SchemaVersion: "1.0", Program: program}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading risk register %q: %w", path, err)
	}
	var r Register
	if err := json.Unmarshal(data, &r); err != nil {
		return nil, fmt.Errorf("parsing risk register %q: %w", path, err)
	}
	return &r, nil
}

// Save writes the register to disk.
func (r *Register) Save(path string) error {
	r.LastUpdated = time.Now().UTC()
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling register: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return fmt.Errorf("creating register directory: %w", err)
	}
	return os.WriteFile(path, data, 0o600)
}

// Note: filepath_dir helper removed — using filepath.Dir directly above.

// Add creates a new risk entry with the next stable RISK-NNN ID.
// Returns the new risk ID.
func (r *Register) Add(risk Risk) string {
	next := r.nextID()
	risk.ID = next
	risk.Created = time.Now().UTC()
	risk.Updated = time.Now().UTC()
	if risk.Status == "" {
		risk.Status = StatusOpen
	}
	risk.Severity = computeSeverity(risk.Likelihood, risk.Impact)
	r.Risks = append(r.Risks, risk)
	return next
}

// nextID returns the next unused RISK-NNN ID.
func (r *Register) nextID() string {
	max := 0
	for _, risk := range r.Risks {
		var n int
		fmt.Sscanf(risk.ID, "RISK-%d", &n)
		if n > max {
			max = n
		}
	}
	return fmt.Sprintf("RISK-%03d", max+1)
}

// computeSeverity derives severity from a 3x3 likelihood/impact matrix.
// Conservative: 3x3 = critical, 3x2 or 2x3 = high, 2x2 = medium, else low.
func computeSeverity(likelihood, impact int) Severity {
	score := likelihood * impact
	switch {
	case score >= 9:
		return SeverityCritical
	case score >= 6:
		return SeverityHigh
	case score >= 4:
		return SeverityMedium
	default:
		return SeverityLow
	}
}

// Filter returns risks matching the given filters.
func (r *Register) Filter(severity, status, owner string) []Risk {
	var out []Risk
	for _, risk := range r.Risks {
		if severity != "" && string(risk.Severity) != severity {
			continue
		}
		if status != "" && string(risk.Status) != status {
			continue
		}
		if owner != "" && risk.Owner != owner {
			continue
		}
		out = append(out, risk)
	}
	return out
}

// SortBySeverity sorts risks from Critical down to Low.
func SortBySeverity(risks []Risk) []Risk {
	order := map[Severity]int{
		SeverityCritical: 0,
		SeverityHigh:     1,
		SeverityMedium:   2,
		SeverityLow:      3,
	}
	sort.Slice(risks, func(i, j int) bool {
		return order[risks[i].Severity] < order[risks[j].Severity]
	})
	return risks
}

// FeedForwardEntry is a single entry from a post-audit feed-forward JSON.
type FeedForwardEntry struct {
	FindingID        string `json:"finding_id"`
	Title            string `json:"title"`
	Description      string `json:"description"`
	ControlID        string `json:"control_id"`
	Severity         string `json:"severity"`
	CorrectiveAction string `json:"corrective_action"`
	Owner            string `json:"owner"`
}

// Update modifies fields on an existing risk entry by ID.
// Only non-zero values in the patch are applied.
func (r *Register) Update(id, newStatus, owner, notes string) error {
	for i := range r.Risks {
		if r.Risks[i].ID == id {
			if newStatus != "" {
				r.Risks[i].Status = Status(newStatus)
			}
			if owner != "" {
				r.Risks[i].Owner = owner
			}
			if notes != "" {
				r.Risks[i].Notes = append(r.Risks[i].Notes, Note{
					Date: time.Now().UTC().Format("2006-01-02"),
					Text: notes,
				})
			}
			r.Risks[i].Updated = time.Now().UTC()
			return nil
		}
	}
	return fmt.Errorf("risk %q not found", id)
}

// Close marks a risk as closed and records the resolution rationale.
func (r *Register) Close(id, resolution string) error {
	for i := range r.Risks {
		if r.Risks[i].ID == id {
			r.Risks[i].Status = StatusClosed
			if resolution != "" {
				r.Risks[i].Notes = append(r.Risks[i].Notes, Note{
					Date: time.Now().UTC().Format("2006-01-02"),
					Text: "RESOLUTION: " + resolution,
				})
			}
			r.Risks[i].Updated = time.Now().UTC()
			return nil
		}
	}
	return fmt.Errorf("risk %q not found", id)
}

// ParseFeedForward parses a post-audit feed-forward JSON without modifying
// the register. Use this for dry-run to count what would be added.
func ParseFeedForward(data []byte) ([]FeedForwardEntry, error) {
	var entries []FeedForwardEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		var wrapper struct {
			Findings []FeedForwardEntry `json:"findings"`
		}
		if err2 := json.Unmarshal(data, &wrapper); err2 != nil {
			return nil, fmt.Errorf("parsing feed-forward JSON: %w", err)
		}
		entries = wrapper.Findings
	}
	return entries, nil
}

// IngestFeedForward reads a post-audit feed-forward JSON and adds each
// finding as a new risk entry linked to the feed-forward source.
func (r *Register) IngestFeedForward(data []byte) ([]string, error) {
	var entries []FeedForwardEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		// Try wrapping object.
		var wrapper struct {
			Findings []FeedForwardEntry `json:"findings"`
		}
		if err2 := json.Unmarshal(data, &wrapper); err2 != nil {
			return nil, fmt.Errorf("parsing feed-forward JSON: %w", err)
		}
		entries = wrapper.Findings
	}

	var ids []string
	for _, e := range entries {
		likelihood, impact := severityToMatrix(e.Severity)
		id := r.Add(Risk{
			Title:           e.Title,
			Description:     e.Description,
			Source:          SourceFeedForward,
			ControlID:       e.ControlID,
			Owner:           e.Owner,
			RemediationPath: e.CorrectiveAction,
			Likelihood:      likelihood,
			Impact:          impact,
			AuditFindingRef: e.FindingID,
			InferenceFlags:  []string{"[INFERRED from post-audit feed-forward]"},
		})
		ids = append(ids, id)
	}
	return ids, nil
}

// severityToMatrix maps a severity string to (likelihood, impact) for the 3x3.
func severityToMatrix(s string) (int, int) {
	switch strings.ToLower(s) {
	case "critical":
		return 3, 3
	case "high":
		return 3, 2
	case "medium":
		return 2, 2
	default:
		return 1, 2
	}
}
