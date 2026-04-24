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
	"context"
	"fmt"
	"log/slog"

	"github.com/urfave/cli/v3"

	"github.com/NVIDIA/aicr/pkg/errors"
)

// skillCmdFlags returns the flags for the skill command.
func skillCmdFlags() []cli.Flag {
	return []cli.Flag{
		withCompletions(&cli.StringFlag{
			Name:     "agent",
			Usage:    "target coding agent",
			Required: true,
		}, supportedAgents),
		&cli.BoolFlag{
			Name:  "stdout",
			Usage: "print generated skill file to stdout instead of writing to disk",
		},
	}
}

// skillCmd returns the CLI command for generating agent skill files.
func skillCmd() *cli.Command {
	return &cli.Command{
		Name:     "skill",
		Category: "Utilities",
		Usage:    "Generate AI agent skill file for AICR CLI.",
		Description: `Generates a skill file that teaches a coding agent
how to use the AICR CLI. The file is written to the agent's
standard configuration directory.

Examples:
  # Generate Claude Code skill file
  aicr skill --agent claude-code

  # Generate Codex skill file
  aicr skill --agent codex

  # Print to stdout instead of writing to disk
  aicr skill --agent claude-code --stdout`,
		Flags:  skillCmdFlags(),
		Action: runSkillCmd,
	}
}

// runSkillCmd generates and writes (or prints) the agent skill file.
func runSkillCmd(_ context.Context, cmd *cli.Command) error {
	agentName := cmd.String("agent")

	at, err := parseAgentType(agentName)
	if err != nil {
		return err
	}

	generators := map[agentType]skillGenerator{
		agentClaudeCode: &claudeCodeGenerator{},
		agentCodex:      &codexGenerator{},
	}

	gen, ok := generators[at]
	if !ok {
		return errors.New(errors.ErrCodeInvalidRequest,
			fmt.Sprintf("no generator registered for agent %q", at))
	}

	slog.Info("generating skill file", "agent", string(at))

	meta := extractCLIMeta(cmd.Root())

	content, err := gen.generate(meta)
	if err != nil {
		return errors.Wrap(errors.ErrCodeInternal, "failed to generate skill file", err)
	}

	if cmd.Bool("stdout") {
		_, err = cmd.Root().Writer.Write(content)
		if err != nil {
			return errors.Wrap(errors.ErrCodeInternal, "failed to write to stdout", err)
		}
		return nil
	}

	path, err := gen.installPath()
	if err != nil {
		return err
	}

	if err := writeSkillFile(path, content); err != nil {
		return err
	}

	fmt.Fprintf(cmd.Root().Writer, "Skill file written to %s\n", path)

	return nil
}
