// Package schema exposes the canonical evidence schema shipped with the CLI.
package schema

import _ "embed"

// CaseV1 is the canonical JSON Schema for case.json.
//
//go:embed case-v1.schema.json
var CaseV1 []byte
