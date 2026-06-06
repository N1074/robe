package stt

import (
	"bytes"
	"context"
	"errors"
	"fmt"
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

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		detail := strings.TrimSpace(stderr.String())
		if detail == "" {
			return "", err
		}
		return "", fmt.Errorf("%w: %s", err, detail)
	}

	transcript := cleanTranscript(stdout.String())
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

func cleanTranscript(output string) string {
	lines := strings.Split(output, "\n")
	kept := make([]string, 0, len(lines))

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || isSTTLogLine(line) {
			continue
		}

		kept = append(kept, line)
	}

	return strings.TrimSpace(strings.Join(kept, " "))
}

func isSTTLogLine(line string) bool {
	prefixes := []string{
		"read_audio_data:",
		"whisper_",
		"whisper.",
		"system_info:",
		"main:",
		"ggml_",
		"common_",
		"audio:",
	}

	for _, prefix := range prefixes {
		if strings.HasPrefix(line, prefix) {
			return true
		}
	}

	return false
}
