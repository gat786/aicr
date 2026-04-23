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
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/urfave/cli/v3"
)

func TestExtractCLIMeta(t *testing.T) {
	root := newRootCmd()
	meta := extractCLIMeta(root)

	if meta.Name != name {
		t.Errorf("name = %q, want %q", meta.Name, name)
	}
	if meta.Version == "" {
		t.Error("version must not be empty")
	}

	// All top-level commands should be captured (snapshot, recipe, query,
	// bundle, verify, validate, trust). Hidden commands and the "skill"
	// command itself (if registered) must be excluded.
	wantCmds := []string{"snapshot", "recipe", "query", "bundle", "verify", "validate", "trust"}
	got := make(map[string]bool)
	for _, c := range meta.Commands {
		got[c.Name] = true
	}
	for _, w := range wantCmds {
		if !got[w] {
			t.Errorf("expected command %q in meta, got commands: %v", w, keys(got))
		}
	}

	// The "skill" command must NOT appear (self-reference avoidance).
	if got["skill"] {
		t.Error("skill command must be excluded from meta")
	}

	// Recipe command must have flags with completions (e.g. --service, --intent).
	var recipeMeta *cmdMeta
	for i := range meta.Commands {
		if meta.Commands[i].Name == "recipe" {
			recipeMeta = &meta.Commands[i]
			break
		}
	}
	if recipeMeta == nil {
		t.Fatal("recipe command not found in meta")
	}

	completionFound := false
	for _, f := range recipeMeta.Flags {
		if len(f.Completions) > 0 {
			completionFound = true
			break
		}
	}
	if !completionFound {
		t.Error("expected at least one flag with completions on recipe command")
	}

	// Verify help and version flags are excluded.
	for _, c := range meta.Commands {
		for _, f := range c.Flags {
			if f.Name == "help" || f.Name == "version" {
				t.Errorf("flag %q must be excluded from command %q", f.Name, c.Name)
			}
		}
	}

	// Trust command should have a subcommand "update".
	var trustMeta *cmdMeta
	for i := range meta.Commands {
		if meta.Commands[i].Name == "trust" {
			trustMeta = &meta.Commands[i]
			break
		}
	}
	if trustMeta == nil {
		t.Fatal("trust command not found in meta")
	}
	subFound := false
	for _, sub := range trustMeta.Subcommands {
		if sub.Name == "update" {
			subFound = true
			break
		}
	}
	if !subFound {
		t.Error("expected trust subcommand 'update' in meta")
	}
}

func TestParseAgentType(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    agentType
		wantErr bool
	}{
		{"claude-code", "claude-code", agentClaudeCode, false},
		{"codex", "codex", agentCodex, false},
		{"empty string", "", "", true},
		{"unknown agent", "gemini", "", true},
		{"case sensitive", "Claude-Code", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseAgentType(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseAgentType(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("parseAgentType(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestSupportedAgents(t *testing.T) {
	agents := supportedAgents()
	if len(agents) == 0 {
		t.Fatal("supportedAgents() must return at least one agent")
	}

	// Verify known agents are present.
	found := make(map[string]bool, len(agents))
	for _, a := range agents {
		found[a] = true
	}
	if !found[string(agentClaudeCode)] {
		t.Error("expected claude-code in supported agents")
	}
	if !found[string(agentCodex)] {
		t.Error("expected codex in supported agents")
	}
}

func TestWriteSkillFile(t *testing.T) {
	dir := t.TempDir()
	// Nested path to verify MkdirAll.
	path := filepath.Join(dir, "nested", "dir", "skill.md")

	content := []byte("# test skill content")
	if err := writeSkillFile(path, content); err != nil {
		t.Fatalf("writeSkillFile() unexpected error: %v", err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read written file: %v", err)
	}
	if string(got) != string(content) {
		t.Errorf("file content = %q, want %q", got, content)
	}
}

func TestWriteSkillFileExists(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "skill.md")

	// Create the file first.
	if err := os.WriteFile(path, []byte("existing"), 0o600); err != nil {
		t.Fatalf("failed to create existing file: %v", err)
	}

	err := writeSkillFile(path, []byte("new content"))
	if err == nil {
		t.Fatal("writeSkillFile() expected error when file exists")
	}
	if !strings.Contains(err.Error(), "skill file already exists") {
		t.Errorf("error message = %q, want containing 'skill file already exists'", err.Error())
	}
	if !strings.Contains(err.Error(), "remove it first to regenerate") {
		t.Errorf("error message = %q, want containing 'remove it first to regenerate'", err.Error())
	}
}

func TestUserHomeDir(t *testing.T) {
	dir, err := userHomeDir()
	if err != nil {
		t.Fatalf("userHomeDir() unexpected error: %v", err)
	}
	if dir == "" {
		t.Error("userHomeDir() returned empty string")
	}
}

func TestSkillInstallPath(t *testing.T) {
	path, err := skillInstallPath(".claude/skills/aicr/SKILL.md")
	if err != nil {
		t.Fatalf("skillInstallPath() unexpected error: %v", err)
	}
	if path == "" {
		t.Error("skillInstallPath() returned empty string")
	}
	// Must end with the expected suffix.
	want := filepath.Join(".claude", "skills", "aicr", "SKILL.md")
	if !strings.HasSuffix(path, want) {
		t.Errorf("skillInstallPath() = %q, want suffix %q", path, want)
	}
}

func TestExtractFlagMeta(t *testing.T) {
	meta := extractCLIMeta(newRootCmd())

	// Find snapshot command to check flag type extraction.
	var snapMeta *cmdMeta
	for i := range meta.Commands {
		if meta.Commands[i].Name == "snapshot" {
			snapMeta = &meta.Commands[i]
			break
		}
	}
	if snapMeta == nil {
		t.Fatal("snapshot command not found in meta")
	}

	// Check that boolean flags are typed correctly.
	flagTypes := make(map[string]string)
	for _, f := range snapMeta.Flags {
		flagTypes[f.Name] = f.Type
	}

	if flagTypes["no-cleanup"] != flagTypeBool {
		t.Errorf("no-cleanup type = %q, want %q", flagTypes["no-cleanup"], flagTypeBool)
	}
	if flagTypes["timeout"] != flagTypeDuration {
		t.Errorf("timeout type = %q, want %q", flagTypes["timeout"], flagTypeDuration)
	}
	if flagTypes["namespace"] != flagTypeString {
		t.Errorf("namespace type = %q, want %q", flagTypes["namespace"], flagTypeString)
	}
}

func TestSkillGenerateClaudeCode(t *testing.T) {
	root := newRootCmd()
	meta := extractCLIMeta(root)

	gen := &claudeCodeGenerator{}
	content, err := gen.generate(meta)
	if err != nil {
		t.Fatalf("generate() unexpected error: %v", err)
	}

	out := string(content)

	// Must start with YAML frontmatter.
	if !strings.HasPrefix(out, "---\n") {
		t.Error("output must start with YAML frontmatter delimiter '---'")
	}

	// Frontmatter fields.
	mustContain := []string{
		"name: aicr",
		"user_invocable: true",
	}
	for _, s := range mustContain {
		if !strings.Contains(out, s) {
			t.Errorf("output missing frontmatter field %q", s)
		}
	}

	// All functional commands must appear.
	wantCmds := []string{"snapshot", "recipe", "query", "bundle", "validate"}
	for _, cmd := range wantCmds {
		if !strings.Contains(out, cmd) {
			t.Errorf("output missing command %q", cmd)
		}
	}

	// Workflow examples.
	wantExamples := []string{
		"aicr snapshot",
		"aicr recipe",
		"aicr bundle",
		"aicr validate",
	}
	for _, ex := range wantExamples {
		if !strings.Contains(out, ex) {
			t.Errorf("output missing workflow example %q", ex)
		}
	}

	// Output format guidance.
	if !strings.Contains(out, "--format json") {
		t.Error("output missing --format json guidance")
	}

	// Prerequisites.
	if !strings.Contains(out, "aicr --version") {
		t.Error("output missing prerequisite 'aicr --version'")
	}

	// Error handling section.
	if !strings.Contains(out, "Error Handling") {
		t.Error("output missing error handling section")
	}

	// Best practices section.
	if !strings.Contains(out, "Best Practices") {
		t.Error("output missing best practices section")
	}

	// Criteria values section should contain dynamic values from recipe flags.
	if !strings.Contains(out, "Criteria Values") {
		t.Error("output missing criteria values section")
	}
}

func TestSkillClaudeCodeInstallPath(t *testing.T) {
	gen := &claudeCodeGenerator{}
	path, err := gen.installPath()
	if err != nil {
		t.Fatalf("installPath() unexpected error: %v", err)
	}

	want := filepath.Join(".claude", "skills", "aicr", "SKILL.md")
	if !strings.HasSuffix(path, want) {
		t.Errorf("installPath() = %q, want suffix %q", path, want)
	}
}

func TestSkillGenerateCodex(t *testing.T) {
	root := newRootCmd()
	meta := extractCLIMeta(root)

	gen := &codexGenerator{}
	content, err := gen.generate(meta)
	if err != nil {
		t.Fatalf("generate() unexpected error: %v", err)
	}

	out := string(content)

	// Must NOT start with YAML frontmatter (Codex uses plain markdown).
	if strings.HasPrefix(out, "---\n") {
		t.Error("output must not start with YAML frontmatter delimiter '---'")
	}

	// Must start with a markdown heading.
	if !strings.HasPrefix(out, "# ") {
		t.Error("output must start with markdown heading '# '")
	}

	// All functional commands must appear.
	wantCmds := []string{"snapshot", "recipe", "query", "bundle", "validate"}
	for _, cmd := range wantCmds {
		if !strings.Contains(out, cmd) {
			t.Errorf("output missing command %q", cmd)
		}
	}

	// Output format guidance.
	if !strings.Contains(out, "--format json") {
		t.Error("output missing --format json guidance")
	}
}

func TestSkillCodexInstallPath(t *testing.T) {
	gen := &codexGenerator{}
	path, err := gen.installPath()
	if err != nil {
		t.Fatalf("installPath() unexpected error: %v", err)
	}

	want := filepath.Join(".codex", "instructions.md")
	if !strings.HasSuffix(path, want) {
		t.Errorf("installPath() = %q, want suffix %q", path, want)
	}
}

func TestSkillCmd_CommandStructure(t *testing.T) {
	cmd := skillCmd()

	if cmd.Name != "skill" {
		t.Errorf("Name = %q, want %q", cmd.Name, "skill")
	}
	if cmd.Category != "Utilities" {
		t.Errorf("Category = %q, want %q", cmd.Category, "Utilities")
	}
	if cmd.Action == nil {
		t.Error("Action must not be nil")
	}

	flagNames := make(map[string]bool)
	for _, f := range cmd.Flags {
		for _, n := range f.Names() {
			flagNames[n] = true
		}
	}
	if !flagNames["agent"] {
		t.Error("expected --agent flag")
	}
	if !flagNames["stdout"] {
		t.Error("expected --stdout flag")
	}
}

func TestSkillCmd_Stdout(t *testing.T) {
	var buf bytes.Buffer

	root := &cli.Command{
		Name:   "aicr",
		Writer: &buf,
		Commands: []*cli.Command{
			skillCmd(),
		},
	}

	err := root.Run(context.Background(), []string{"aicr", "skill", "--agent", "claude-code", "--stdout"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "aicr") {
		t.Error("output must contain 'aicr'")
	}
	if !strings.HasPrefix(out, "---\n") {
		t.Error("output must start with YAML frontmatter '---'")
	}
}

func TestSkillCmd_MissingAgent(t *testing.T) {
	var buf bytes.Buffer

	root := &cli.Command{
		Name:   "aicr",
		Writer: &buf,
		Commands: []*cli.Command{
			skillCmd(),
		},
	}

	err := root.Run(context.Background(), []string{"aicr", "skill"})
	if err == nil {
		t.Fatal("expected error when --agent is missing")
	}
}

// keys returns sorted map keys for diagnostic output.
func keys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
