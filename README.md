# specimen

Every risk gets a stable `RISK-NNN` identifier. None are reused after deletion. Severity is derived, not set — pass likelihood and impact, `specimen` computes the rest.

```bash
go get github.com/Formulary-Labs/specimen
```

## What it does

`specimen` manages a program's risk register as a JSON file at `data/[program]/risks.json`. It assigns stable `RISK-NNN` identifiers, derives severity from a 3×3 likelihood/impact matrix, and supports bulk ingestion of post-audit findings via feed-forward.

## Usage

```go
import "github.com/Formulary-Labs/specimen/register"

// Load existing register (returns empty register if file does not exist)
reg, err := register.Load("data/my-program/risks.json", "my-program")

// Add a risk
id, err := reg.Add(register.Risk{
    Title:           "Evidence collection backlog",
    Description:     "Three control families have no linked evidence for the current cycle.",
    Source:          register.SourceCoverageGap,
    Likelihood:      2,
    Impact:          3,
    Owner:           "security-team",
    ControlID:       "A.12.1",
    RemediationPath: "Schedule evidence review sprint before Q4 audit.",
})
// id = "RISK-004"

// Append a note to an existing risk
err = reg.AddNote("RISK-004", register.Note{
    Date: "2026-09-18",
    Text: "Sprint scheduled for October 6.",
})

err = reg.Save("data/my-program/risks.json")
```

## Risk fields

| Field | Type | Notes |
|---|---|---|
| `id` | string | Stable `RISK-NNN`, assigned on `Add` |
| `title` | string | Short description |
| `description` | string | Full description |
| `source` | string | How the risk was identified — see sources |
| `likelihood` | int | 1–3 |
| `impact` | int | 1–3 |
| `severity` | string | Derived — never set directly |
| `status` | string | `open`, `accepted`, `mitigated`, `closed` |
| `owner` | string | Responsible party |
| `control_id` | string | Associated control (optional) |
| `evidence_ref` | string | Evidence reference (optional) |
| `remediation_path` | string | Planned remediation |
| `accept_rationale` | string | Required when `status` is `accepted` |
| `audit_finding_ref` | string | Source audit finding ID, for feed-forward entries |

### Risk sources

| Source | Meaning |
|---|---|
| `coverage_gap` | Identified from a titer coverage gap |
| `evidence_gap` | Identified from a missing evidence link |
| `owner_gap` | Identified from an unowned control |
| `explicit` | Manually logged |
| `inferred` | Agent-inferred from program state |
| `feed_forward` | Ingested from a post-audit feed-forward artifact |

## Severity matrix

Severity is derived from `likelihood × impact`:

| Likelihood × Impact | Severity |
|---|---|
| ≥ 9 | `critical` |
| ≥ 6 | `high` |
| ≥ 4 | `medium` |
| < 4 | `low` |

Do not set `severity` directly. Set `likelihood` and `impact`; `specimen` computes severity on `Add` and on any subsequent `Update` that changes either field.

## Filtering and sorting

```go
// Filter by severity, status, and owner (empty string = no filter on that field)
risks := reg.Filter("high", "open", "security-team")

// Sort critical → low
sorted := reg.SortBySeverity(risks)

// All open and in-remediation risks regardless of severity or owner
all_open := reg.Filter("", "open", "")
```

## Post-audit feed-forward

`IngestFeedForward` bulk-ingests findings from a post-audit feed-forward JSON artifact. Each finding becomes a new risk entry with source `feed_forward` and a new stable `RISK-NNN` ID. IDs from the prior register are never reused.

```go
err := reg.IngestFeedForward(feedForwardJSON)
err = reg.Save("data/my-program/risks.json")
```

This closes the audit-to-improvement loop: `decay` flags recurring findings, the agent layer produces the feed-forward artifact, `specimen` ingests it as actionable risk entries in the next cycle.

## Pipeline context

`specimen` sits at the center of the risk tracking loop:

- Receives gaps from `titer` (coverage, owner, evidence gaps → risk entries)
- Receives scan findings from `scan` (high-relevance external items → risk entries)
- Receives feed-forward from post-audit processing
- Feeds `vital` (risk dimension of the health snapshot)
- Feeds `exhibit` (risk register section of the auditor dashboard)
- Feeds `formula` (risk CSVs for artifact generation)

## License

Apache License 2.0
