// Copyright (c) 2026, NVIDIA CORPORATION.  All rights reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cli

import (
	"bytes"
	"fmt"
)

// Compile-time interface check.
var _ skillGenerator = (*codexGenerator)(nil)

// codexGenerator produces a Codex instructions.md from CLI metadata.
type codexGenerator struct{}

func (g *codexGenerator) generate(meta *cliMeta) ([]byte, error) {
	var buf bytes.Buffer

	// No frontmatter — plain markdown.
	fmt.Fprintf(&buf, "# AICR CLI (%s)\n\n", meta.Version)
	fmt.Fprintf(&buf, "NVIDIA AI Cluster Runtime — generates validated GPU-accelerated Kubernetes configurations.\n\n")

	writePrerequisites(&buf)
	writeCommandReference(&buf, meta)
	writeCriteriaValues(&buf, meta)
	writeOutputFormatGuidance(&buf)
	writeWorkflowExamples(&buf)
	writeErrorHandling(&buf)
	writeBestPractices(&buf)

	return buf.Bytes(), nil
}

func (g *codexGenerator) installPath() (string, error) {
	return skillInstallPath(".codex/instructions.md")
}
