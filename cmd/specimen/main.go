// specimen manages the compliance risk register and POA&M.
//
// Subcommands:
//
//	specimen add     Add a risk entry to the register
//	specimen list    List and filter risk entries
//	specimen ingest  Ingest post-audit feed-forward JSON
//	specimen status  Show register summary
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/Formulary-Labs/specimen/register"
	"github.com/Formulary-Labs/substrate/exit"
	"github.com/Formulary-Labs/substrate/provenance"
)

const version = "0.1.0"

func registerPath(program string) string {
	if program == "" {
		return "risks.json"
	}
	return filepath.Join("data", program, "risks.json")
}

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(exit.ToolError)
	}

	switch os.Args[1] {
	case "add":
		runAdd(os.Args[2:])
	case "list":
		runList(os.Args[2:])
	case "ingest":
		runIngest(os.Args[2:])
	case "status":
		runStatus(os.Args[2:])
	case "--version", "-v", "version":
		fmt.Printf("specimen version %s\n", version)
	case "--help", "-h", "help":
		printUsage()
	default:
		fmt.Fprintf(os.Stderr, "unknown subcommand: %q\n", os.Args[1])
		printUsage()
		os.Exit(exit.ToolError)
	}
}

func runAdd(args []string) {
	fs := flag.NewFlagSet("add", flag.ExitOnError)
	var (
		program    = fs.String("program", "", "Program slug")
		title      = fs.String("title", "", "Risk title (required)")
		desc       = fs.String("description", "", "Risk description")
		source     = fs.String("source", "explicit", "Source: coverage_gap, evidence_gap, owner_gap, explicit, inferred")
		likelihood = fs.Int("likelihood", 2, "Likelihood 1-3")
		impact     = fs.Int("impact", 2, "Impact 1-3")
		owner      = fs.String("owner", "", "Risk owner")
		controlID  = fs.String("control", "", "Related control ID")
		dryRun     = fs.Bool("dry-run", false, "Print what would be added without writing")
	)
	fs.Parse(args) //nolint:errcheck

	if *title == "" {
		fmt.Fprintln(os.Stderr, "error: --title is required")
		os.Exit(exit.ToolError)
	}

	path := registerPath(*program)
	reg, err := register.Load(path, *program)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error loading register: %v\n", err)
		os.Exit(exit.ToolError)
	}

	risk := register.Risk{
		Title:       *title,
		Description: *desc,
		Source:      register.Source(*source),
		Likelihood:  *likelihood,
		Impact:      *impact,
		Owner:       *owner,
		ControlID:   *controlID,
	}

	if *dryRun {
		risk.ID = "RISK-DRY"
		data, _ := json.MarshalIndent(risk, "", "  ")
		fmt.Println(string(data))
		return
	}

	id := reg.Add(risk)
	if err := reg.Save(path); err != nil {
		fmt.Fprintf(os.Stderr, "error saving register: %v\n", err)
		os.Exit(exit.ToolError)
	}

	fmt.Printf(`{"id": %q, "severity": %q, "status": "open"}`+"\n", id, reg.Risks[len(reg.Risks)-1].Severity)

	_ = provenance.Write("logs/provenance.jsonl", provenance.Entry{
		Spec:        "functions/risk-register-spec.md",
		Output:      path,
		OutputType:  "other",
		Program:     *program,
		Purpose:     fmt.Sprintf("specimen add: %s — %s", id, *title),
		Reusability: provenance.Instance,
		QualityGate: provenance.Pass,
		Tool:        "specimen",
		ToolVersion: version,
	})
}

func runList(args []string) {
	fs := flag.NewFlagSet("list", flag.ExitOnError)
	var (
		program  = fs.String("program", "", "Program slug")
		severity = fs.String("severity", "", "Filter by severity: critical, high, medium, low")
		status   = fs.String("status", "", "Filter by status: open, accepted, mitigated, closed")
		owner    = fs.String("owner", "", "Filter by owner")
		fmtFlag  = fs.String("format", "json", "Output format: json, md")
	)
	fs.Parse(args) //nolint:errcheck

	path := registerPath(*program)
	reg, err := register.Load(path, *program)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error loading register: %v\n", err)
		os.Exit(exit.ToolError)
	}

	risks := register.SortBySeverity(reg.Filter(*severity, *status, *owner))

	if *fmtFlag == "md" {
		printMD(risks)
		return
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	enc.Encode(map[string]interface{}{ //nolint:errcheck
		"total": len(risks),
		"risks": risks,
	})
}

func printMD(risks []register.Risk) {
	fmt.Printf("# Risk Register\n\n**%d items**\n\n", len(risks))
	fmt.Println("| ID | Title | Severity | Status | Owner | Control |")
	fmt.Println("|---|---|---|---|---|---|")
	for _, r := range risks {
		fmt.Printf("| %s | %s | %s | %s | %s | %s |\n",
			r.ID, r.Title, r.Severity, r.Status, r.Owner, r.ControlID)
	}
}

func runIngest(args []string) {
	fs := flag.NewFlagSet("ingest", flag.ExitOnError)
	var (
		program      = fs.String("program", "", "Program slug")
		feedForward  = fs.String("feed-forward", "", "Path to post-audit feed-forward JSON (required)")
		dryRun       = fs.Bool("dry-run", false, "Print what would be ingested without writing")
	)
	fs.Parse(args) //nolint:errcheck

	if *feedForward == "" {
		fmt.Fprintln(os.Stderr, "error: --feed-forward is required")
		os.Exit(exit.ToolError)
	}

	data, err := os.ReadFile(*feedForward)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error reading feed-forward file: %v\n", err)
		os.Exit(exit.ToolError)
	}

	path := registerPath(*program)
	reg, err := register.Load(path, *program)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error loading register: %v\n", err)
		os.Exit(exit.ToolError)
	}

	ids, err := reg.IngestFeedForward(data)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error ingesting feed-forward: %v\n", err)
		os.Exit(exit.ToolError)
	}

	if *dryRun {
		fmt.Printf(`{"dry_run": true, "would_add": %d}`+"\n", len(ids))
		return
	}

	if err := reg.Save(path); err != nil {
		fmt.Fprintf(os.Stderr, "error saving register: %v\n", err)
		os.Exit(exit.ToolError)
	}

	fmt.Printf(`{"ingested": %d, "ids": %v}`+"\n", len(ids), mustJSON(ids))

	_ = provenance.Write("logs/provenance.jsonl", provenance.Entry{
		Spec:        "functions/risk-register-spec.md",
		Output:      path,
		OutputType:  "other",
		Program:     *program,
		Purpose:     fmt.Sprintf("specimen ingest: %d risks from feed-forward %s", len(ids), *feedForward),
		Reusability: provenance.Instance,
		QualityGate: provenance.Pass,
		Tool:        "specimen",
		ToolVersion: version,
	})
}

func runStatus(args []string) {
	fs := flag.NewFlagSet("status", flag.ExitOnError)
	program := fs.String("program", "", "Program slug")
	fs.Parse(args) //nolint:errcheck

	path := registerPath(*program)
	reg, err := register.Load(path, *program)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error loading register: %v\n", err)
		os.Exit(exit.ToolError)
	}

	counts := map[string]int{"critical": 0, "high": 0, "medium": 0, "low": 0}
	statusCounts := map[string]int{"open": 0, "accepted": 0, "mitigated": 0, "closed": 0}
	for _, r := range reg.Risks {
		counts[string(r.Severity)]++
		statusCounts[string(r.Status)]++
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	enc.Encode(map[string]interface{}{ //nolint:errcheck
		"program":      reg.Program,
		"total":        len(reg.Risks),
		"by_severity":  counts,
		"by_status":    statusCounts,
		"last_updated": reg.LastUpdated,
	})
}

func mustJSON(v interface{}) string {
	data, _ := json.Marshal(v)
	return string(data)
}

func printUsage() {
	fmt.Fprintln(os.Stderr, `specimen — compliance risk register and POA&M

Usage:
  specimen <subcommand> [flags]

Subcommands:
  add      Add a risk entry (--title required)
  list     List and filter risks (--severity, --status, --owner)
  ingest   Ingest post-audit feed-forward JSON (--feed-forward required)
  status   Show register summary

Examples:
  specimen add --program iso42001 --title "No MFA on admin accounts" --severity high
  specimen list --program iso42001 --severity critical --format md
  specimen ingest --program iso42001 --feed-forward post-audit/2026-feed-forward.json
  specimen status --program iso42001`)
}
