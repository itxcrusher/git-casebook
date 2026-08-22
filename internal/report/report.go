package report

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode"

	"github.com/itxcrusher/repo-rehab/internal/model"
)

func Render(c model.Case) []byte {
	var out bytes.Buffer
	fmt.Fprintln(&out, "# Forensic repository case")
	fmt.Fprintln(&out)
	fmt.Fprintf(&out, "- Case ID: `%s`\n", escape(c.CaseID))
	fmt.Fprintf(&out, "- Status: **%s**\n", escape(c.Status))
	fmt.Fprintf(&out, "- Semantic fingerprint: `%s`\n", escape(c.CaseFingerprint))
	fmt.Fprintf(&out, "- Tool: `%s` using `%s`\n", escape(c.Tool.Version), escape(c.Tool.GitVersion))
	fmt.Fprintln(&out, "- Authority: generated from `case.json`; this report is not authoritative state.")
	fmt.Fprintln(&out)

	fmt.Fprintln(&out, "## Sources")
	fmt.Fprintln(&out)
	fmt.Fprintln(&out, "| Source | Locator | Role | Integrity | Completeness | Refs | Commits | Reachable objects | Default branch |")
	fmt.Fprintln(&out, "| --- | --- | --- | --- | --- | ---: | ---: | ---: | --- |")
	for _, source := range c.Sources {
		defaultBranch := "(not observed)"
		if source.DefaultBranch != nil {
			defaultBranch = "`" + escape(*source.DefaultBranch) + "`"
		}
		fmt.Fprintf(&out, "| `%s` | `%s` | %s | %s | %s | %d | %d | %d | %s |\n",
			escape(source.SourceID), escape(source.OriginalLocator), escape(source.Role), escape(source.Integrity), escape(source.Completeness),
			len(source.Refs), source.Objects.CommitCount, source.Objects.ReachableObjectCount, defaultBranch)
	}
	fmt.Fprintln(&out)

	fmt.Fprintln(&out, "## Provenance relationships")
	fmt.Fprintln(&out)
	if len(c.Relationships) == 0 {
		fmt.Fprintln(&out, "No pairwise relationship exists for a single-source case.")
	} else {
		fmt.Fprintln(&out, "| Direction | Classification | Shared commits | A-only commits | B-only commits | Evidence |")
		fmt.Fprintln(&out, "| --- | --- | ---: | ---: | ---: | --- |")
		for _, rel := range c.Relationships {
			evidenceID := ""
			if len(rel.EvidenceIDs) > 0 {
				evidenceID = "`" + escape(rel.EvidenceIDs[0]) + "`"
			}
			fmt.Fprintf(&out, "| `%s` -> `%s` | **%s** | %d | %d | %d | %s |\n",
				escape(rel.SourceA), escape(rel.SourceB), escape(rel.Classification), rel.SharedCommitCount,
				rel.SourceAOnlyCommitCount, rel.SourceBOnlyCommitCount, evidenceID)
		}
	}
	fmt.Fprintln(&out)
	if len(c.Findings) > 0 {
		fmt.Fprintln(&out, "## Findings and uncertainty")
		fmt.Fprintln(&out)
		for _, finding := range c.Findings {
			fmt.Fprintf(&out, "- `%s` [%s]: %s\n", escape(finding.FindingID), escape(finding.Kind), escape(finding.Summary))
		}
		fmt.Fprintln(&out)
	}

	refCount, collisions, review := 0, 0, 0
	for _, source := range c.Sources {
		for _, ref := range source.Refs {
			refCount++
			if ref.CollisionState == "COLLISION" {
				collisions++
			}
			if ref.ArchivalDisposition == "REVIEW" {
				review++
			}
		}
	}
	fmt.Fprintln(&out, "## Archival ref plan")
	fmt.Fprintln(&out)
	fmt.Fprintf(&out, "- Source refs inventoried: %d\n", refCount)
	fmt.Fprintf(&out, "- Refs in mapping collisions: %d\n", collisions)
	fmt.Fprintf(&out, "- Refs requiring review: %d\n", review)
	fmt.Fprintln(&out, "- This is a plan only. No destination ref was created or pushed.")
	fmt.Fprintln(&out)

	fmt.Fprintln(&out, "## Closed gates")
	fmt.Fprintln(&out)
	gates := append([]model.Gate(nil), c.Gates...)
	sort.Slice(gates, func(i, j int) bool { return gates[i].GateID < gates[j].GateID })
	for _, gate := range gates {
		fmt.Fprintf(&out, "- `%s`: **%s** - %s\n", escape(gate.Action), escape(gate.State), escape(gate.Reason))
	}
	fmt.Fprintln(&out)

	fmt.Fprintln(&out, "## Recommended next human action")
	fmt.Fprintln(&out)
	if hasIncomplete(c) {
		fmt.Fprintln(&out, "Resolve the recorded incomplete or untrusted source evidence before selecting a canonical history. Treat every affected relationship as `UNKNOWN`.")
	} else if collisions > 0 || review > 0 {
		fmt.Fprintln(&out, "Review the withheld archival mappings. Do not apply a plan until every collision or unsupported ref has an explicit disposition.")
	} else {
		fmt.Fprintln(&out, "Review the deterministic relationship evidence and proposed ref plan. Canonical selection and any later remote write remain human decisions.")
	}
	fmt.Fprintln(&out)
	fmt.Fprintln(&out, "Repository metadata and generated case evidence may be sensitive. Review before sharing.")
	return out.Bytes()
}

func Write(path string, content []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	return os.WriteFile(path, content, 0o600)
}

func hasIncomplete(c model.Case) bool {
	for _, source := range c.Sources {
		if source.Integrity != "VERIFIED" || source.Completeness != "COMPLETE" {
			return true
		}
	}
	for _, rel := range c.Relationships {
		if rel.Classification == "UNKNOWN" {
			return true
		}
	}
	return false
}

func escape(value string) string {
	value = strings.ToValidUTF8(value, "�")
	value = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return '�'
		}
		return r
	}, value)
	replacer := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", "|", "&#124;", "`", "&#96;")
	return replacer.Replace(value)
}
