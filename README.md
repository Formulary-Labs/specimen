# specimen

Every risk gets a stable `RISK-NNN` identifier. None are reused after deletion. Severity is derived, not set — pass likelihood and impact, `specimen` computes the rest.

```bash
go get github.com/Formulary-Labs/specimen
```

## In the ecosystem

Formulary is a **pick-and-choose** suite of small, single-purpose CLIs. Install only the tools your workflow needs; each runs standalone. Shared contracts live in [`substrate`](https://github.com/Formulary-Labs/substrate). Optional Make pipelines (or an agent such as `regimen`) compose tools into a run — plug-and-play, not all-or-nothing.

**This tool:** risk register with stable RISK-NNN IDs and feed-forward ingest.  
**Works with:** `antidote --emit specimen` upstream; `exhibit` / `vital` downstream.  
**Pipeline stage:** packet / close.


## What it does

`specimen` manages a program's risk register as a JSON file at `data/[program]/risks.json`. It assigns stable `RISK-NNN` identifiers, derives severity from a 3×3 likelihood/impact matrix, and supports bulk ingestion of post-audit findings via feed-forward.

## CLI

`specimen` is a full risk-register CLI. Severity is always derived from likelihood × impact — it is never set directly:

```bash
# Add a risk (severity derived, stable RISK-NNN assigned)
specimen add --program my-isms --title "Evidence backlog" \
  --source coverage_gap --likelihood 2 --impact 3 --owner security-team

# Update with FAIR ALE derivation from an org loss table
specimen update --program my-isms --id RISK-004 --status accepted \
  --loss-table org-loss-table.yaml --data-class PII --record-count 50000 \
  --notes "Accepted for Q4; compensating control in place"

# Close with resolution rationale
specimen close --program my-isms --id RISK-002 --resolution "Control implemented and evidenced"

# Elevate satisfied assessment controls into risks (from an assay EvaluationLog)
specimen elevate --program my-isms --evaluation-log output/assessment.json

# Bulk-ingest post-audit feed-forward (antidote --emit specimen output)
specimen ingest --program my-isms --feed-forward out/feed-forward.json

# Ingest a gemara RiskCatalog as register entries
specimen catalog --program my-isms --catalog risks.yaml

# List with filters
specimen list --program my-isms --severity high --status open --format md

# Register summary
specimen status --program my-isms
```

`add`, `ingest`, and `catalog` support `--dry-run` (print what would be written without writing). `update --loss-table` derives `AssetValue` (FAIR ALE) from the shared org loss table in `substrate/losstable`, using `--data-class` and `--record-count`.

## Library usage

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
