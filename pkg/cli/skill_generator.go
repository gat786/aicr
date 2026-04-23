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
	"fmt"
	"os"
	"path/filepath"

	"github.com/urfave/cli/v3"

	"github.com/NVIDIA/aicr/pkg/errors"
)

// agentType identifies a supported coding agent.
type agentType string

const (
	agentClaudeCode agentType = "claude-code"
	agentCodex      agentType = "codex"
)

// supportedAgents returns the string names of all recognized agent types.
// The return type is []string so it can be passed directly to withCompletions.
func supportedAgents() []string {
	return []string{string(agentClaudeCode), string(agentCodex)}
}

// parseAgentType validates and returns the agentType for the given string.
func parseAgentType(s string) (agentType, error) {
	for _, a := range supportedAgents() {
		if s == a {
			return agentType(a), nil
		}
	}
	return "", errors.New(errors.ErrCodeInvalidRequest,
		fmt.Sprintf("unsupported agent type %q: must be one of %v", s, supportedAgents()))
}

// skillGenerator produces an agent-specific skill file from CLI metadata.
// Implementations are registered per agentType (see claude_code_generator.go,
// codex_generator.go).
type skillGenerator interface {
	// generate renders the skill file content from the given CLI metadata.
	generate(meta *cliMeta) ([]byte, error)
	// installPath returns the absolute path where the skill file should be written.
	installPath() (string, error)
}

// cliMeta captures the full CLI command tree for skill generation.
type cliMeta struct {
	Name     string
	Version  string
	Commands []cmdMeta
}

// cmdMeta captures a single CLI command.
type cmdMeta struct {
	Name        string
	Usage       string
	Description string
	Flags       []flagMeta
	Subcommands []cmdMeta
}

const (
	flagTypeString      = "string"
	flagTypeBool        = "bool"
	flagTypeInt         = "int"
	flagTypeDuration    = "duration"
	flagTypeStringSlice = "stringSlice"
)

// flagMeta captures a single CLI flag.
type flagMeta struct {
	Name        string
	Aliases     []string
	Usage       string
	Type        string
	Default     string
	Required    bool
	Completions []string
}

// extractCLIMeta walks the urfave/cli command tree rooted at root and returns
// a cliMeta snapshot. Hidden commands and the "skill" command itself are excluded.
func extractCLIMeta(root *cli.Command) *cliMeta {
	meta := &cliMeta{
		Name:    root.Name,
		Version: version,
	}

	for _, cmd := range root.Commands {
		if cmd.Hidden || cmd.Name == "skill" {
			continue
		}
		meta.Commands = append(meta.Commands, extractCmdMeta(cmd))
	}

	return meta
}

// extractCmdMeta builds a cmdMeta from a single urfave/cli command,
// recursively extracting subcommands and flags.
func extractCmdMeta(cmd *cli.Command) cmdMeta {
	m := cmdMeta{
		Name:        cmd.Name,
		Usage:       cmd.Usage,
		Description: cmd.Description,
	}

	for _, f := range cmd.Flags {
		fm := extractFlagMeta(f)
		if fm == nil {
			continue
		}
		m.Flags = append(m.Flags, *fm)
	}

	for _, sub := range cmd.Commands {
		if sub.Hidden {
			continue
		}
		m.Subcommands = append(m.Subcommands, extractCmdMeta(sub))
	}

	return m
}

// extractFlagMeta converts a urfave/cli Flag into a flagMeta.
// Returns nil for help and version flags which are auto-generated.
func extractFlagMeta(f cli.Flag) *flagMeta {
	names := f.Names()
	if len(names) == 0 {
		return nil
	}

	primary := names[0]
	if primary == "help" || primary == "version" {
		return nil
	}

	fm := &flagMeta{
		Name: primary,
	}

	if len(names) > 1 {
		fm.Aliases = names[1:]
	}

	switch tf := f.(type) {
	case *cli.StringFlag:
		fm.Type = flagTypeString
		fm.Usage = tf.Usage
		fm.Default = tf.Value
		fm.Required = tf.Required
	case *cli.BoolFlag:
		fm.Type = flagTypeBool
		fm.Usage = tf.Usage
		if tf.Value {
			fm.Default = "true"
		}
	case *cli.IntFlag:
		fm.Type = flagTypeInt
		fm.Usage = tf.Usage
		if tf.Value != 0 {
			fm.Default = fmt.Sprintf("%d", tf.Value)
		}
	case *cli.DurationFlag:
		fm.Type = flagTypeDuration
		fm.Usage = tf.Usage
		if tf.Value != 0 {
			fm.Default = tf.Value.String()
		}
	case *cli.StringSliceFlag:
		fm.Type = flagTypeStringSlice
		fm.Usage = tf.Usage
	default:
		// Handle wrapped types (e.g. completableStringFlag).
		fm.Type = flagTypeString
		if u, ok := f.(interface{ GetUsage() string }); ok {
			fm.Usage = u.GetUsage()
		}
	}

	// Check if flag provides shell completions.
	if cf, ok := f.(CompletableFlag); ok {
		fm.Completions = cf.Completions()
	}

	return fm
}

// userHomeDir returns the current user's home directory.
func userHomeDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", errors.Wrap(errors.ErrCodeInternal, "failed to determine home directory", err)
	}
	return home, nil
}

// skillInstallPath builds an absolute path relative to the user's home directory.
// Each generator provides its own relative path (e.g. ".claude/skills/aicr/SKILL.md").
func skillInstallPath(relPath string) (string, error) {
	home, err := userHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, filepath.FromSlash(relPath)), nil
}

// writeSkillFile writes content to the given path, creating parent directories
// as needed. Returns an error if the file already exists.
func writeSkillFile(path string, content []byte) error {
	if _, err := os.Stat(path); err == nil {
		return errors.New(errors.ErrCodeInvalidRequest,
			fmt.Sprintf("skill file already exists: %s (remove it first to regenerate)", path))
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return errors.Wrap(errors.ErrCodeInternal,
			fmt.Sprintf("failed to create directory %s", dir), err)
	}

	if err := os.WriteFile(path, content, 0o600); err != nil {
		return errors.Wrap(errors.ErrCodeInternal,
			fmt.Sprintf("failed to write skill file %s", path), err)
	}

	return nil
}
