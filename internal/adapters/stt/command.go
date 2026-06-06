package stt

import (
	"context"
	"errors"
	"os/exec"
	"strings"
	"time"
)

type CommandTranscriber struct {
	command string
	args    []string
	timeout time.Duration
}

func NewCommandTranscriber(command string, args []string, timeout time.Duration) *CommandTranscriber {
	return &CommandTranscriber{
		command: strings.TrimSpace(command),
		args:    args,
		timeout: timeout,
	}
}

func (t *CommandTranscriber) Transcribe(ctx context.Context, audioPath string) (string, error) {
	if t.command == "" {
		return "", errors.New("stt command is required")
	}
	if strings.TrimSpace(audioPath) == "" {
		return "", errors.New("audio path is required")
	}

	if t.timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, t.timeout)
		defer cancel()
	}

	args := buildArgs(t.args, audioPath)
	cmd := exec.CommandContext(ctx, t.command, args...)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", err
	}

	transcript := strings.TrimSpace(string(output))
	if transcript == "" {
		return "", errors.New("stt command returned empty transcript")
	}

	return transcript, nil
}

func buildArgs(args []string, audioPath string) []string {
	out := make([]string, 0, len(args)+1)
	replaced := false

	for _, arg := range args {
		if strings.Contains(arg, "{audio}") {
			out = append(out, strings.ReplaceAll(arg, "{audio}", audioPath))
			replaced = true
			continue
		}

		out = append(out, arg)
	}

	if !replaced {
		out = append(out, audioPath)
	}

	return out
}
