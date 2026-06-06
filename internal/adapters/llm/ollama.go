package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/N1074/robe/internal/core"
)

type OllamaClient struct {
	baseURL     string
	model       string
	numPredict  int
	temperature float64
	httpClient  *http.Client
}

func NewOllamaClient(baseURL string, model string, numPredict int, temperature float64) *OllamaClient {
	return &OllamaClient{
		baseURL:     strings.TrimRight(baseURL, "/"),
		model:       model,
		numPredict:  numPredict,
		temperature: temperature,
		httpClient: &http.Client{
			Timeout: 90 * time.Second,
		},
	}
}

func (c *OllamaClient) Ask(ctx context.Context, prompt string) (string, error) {
	if strings.TrimSpace(prompt) == "" {
		return "", errors.New("prompt is empty")
	}

	reqBody := chatRequest{
		Model: c.model,
		Messages: []chatMessage{
			{
				Role:    "system",
				Content: "Eres Robe, un asistente personal local. Responde de forma directa, útil y concisa. No muestres razonamiento interno.",
			},
			{
				Role:    "user",
				Content: "/no_think\n" + prompt,
			},
		},
		Stream: false,
		Options: chatOptions{
			NumPredict:  c.numPredict,
			Temperature: c.temperature,
		},
	}

	payload, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/api/chat", bytes.NewReader(payload))
	if err != nil {
		return "", err
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("ollama returned status %d", resp.StatusCode)
	}

	var out chatResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", err
	}

	content := stripThinking(out.Message.Content)
	if content == "" {
		return "", errors.New("ollama returned empty content")
	}

	return content, nil
}

func (c *OllamaClient) ParseIntent(ctx context.Context, req core.IntentRequest) (core.Intent, error) {
	if strings.TrimSpace(req.Text) == "" {
		return core.Intent{Kind: core.IntentNone}, nil
	}

	reqBody := chatRequest{
		Model: c.model,
		Messages: []chatMessage{
			{
				Role:    "system",
				Content: intentSystemPrompt(),
			},
			{
				Role: "user",
				Content: fmt.Sprintf(
					"/no_think\nNow: %s\nTimezone: %s\nUser text: %s",
					req.Now.Format(time.RFC3339),
					req.Timezone,
					req.Text,
				),
			},
		},
		Stream: false,
		Options: chatOptions{
			NumPredict:  384,
			Temperature: 0,
		},
	}

	payload, err := json.Marshal(reqBody)
	if err != nil {
		return core.Intent{}, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/api/chat", bytes.NewReader(payload))
	if err != nil {
		return core.Intent{}, err
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return core.Intent{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return core.Intent{}, fmt.Errorf("ollama returned status %d", resp.StatusCode)
	}

	var out chatResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return core.Intent{}, err
	}

	return decodeIntent(stripThinking(out.Message.Content))
}

func intentSystemPrompt() string {
	return `You are Robe's intent parser. Return only compact JSON. No markdown.

Supported actions:
- calendar_list: user asks to see calendar events. Use period "today", "tomorrow", or "week".
- calendar_create: user asks to create an appointment/event. Fill title, start, end, location, description. If duration/end is missing, use one hour. Resolve relative dates from Now and Timezone.
- calendar_delete: user asks to delete a calendar event only when an explicit event id is present. Fill event_id.
- ask: ordinary assistant question.
- none: empty or unclear.

Rules:
- Never confirm or execute writes.
- Never invent an event_id for deletion.
- Times must be RFC3339.
- JSON shape:
{"action":"calendar_create","title":"Dentist","start":"2026-06-07T12:00:00+02:00","end":"2026-06-07T13:00:00+02:00","location":"","description":""}
{"action":"calendar_delete","event_id":"abc123"}
{"action":"calendar_list","period":"today"}
{"action":"ask","prompt":"..."}
{"action":"none"}`
}

type intentResponse struct {
	Action      string `json:"action"`
	Prompt      string `json:"prompt"`
	Period      string `json:"period"`
	Title       string `json:"title"`
	Start       string `json:"start"`
	End         string `json:"end"`
	Location    string `json:"location"`
	Description string `json:"description"`
	EventID     string `json:"event_id"`
}

func decodeIntent(content string) (core.Intent, error) {
	content = extractJSONObject(content)

	var parsed intentResponse
	if err := json.Unmarshal([]byte(content), &parsed); err != nil {
		return core.Intent{}, err
	}

	switch strings.TrimSpace(parsed.Action) {
	case core.IntentCalendarList:
		return core.Intent{
			Kind:           core.IntentCalendarList,
			CalendarPeriod: strings.TrimSpace(parsed.Period),
		}, nil

	case core.IntentCalendarCreate:
		start, err := parseIntentTime(parsed.Start)
		if err != nil {
			return core.Intent{}, err
		}
		end, err := parseIntentTime(parsed.End)
		if err != nil {
			return core.Intent{}, err
		}

		return core.Intent{
			Kind: core.IntentCalendarCreate,
			CalendarDraft: core.CalendarEventDraft{
				Title:       strings.TrimSpace(parsed.Title),
				Start:       start,
				End:         end,
				Location:    strings.TrimSpace(parsed.Location),
				Description: strings.TrimSpace(parsed.Description),
			},
		}, nil

	case core.IntentCalendarDelete:
		return core.Intent{
			Kind:            core.IntentCalendarDelete,
			CalendarEventID: strings.TrimSpace(parsed.EventID),
		}, nil

	case core.IntentAsk:
		return core.Intent{
			Kind:      core.IntentAsk,
			AskPrompt: strings.TrimSpace(parsed.Prompt),
		}, nil

	default:
		return core.Intent{Kind: core.IntentNone}, nil
	}
}

func parseIntentTime(value string) (time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}, errors.New("intent time is empty")
	}

	return time.Parse(time.RFC3339, value)
}

func extractJSONObject(content string) string {
	content = strings.TrimSpace(content)

	start := strings.Index(content, "{")
	end := strings.LastIndex(content, "}")
	if start == -1 || end == -1 || end < start {
		return content
	}

	return content[start : end+1]
}

func stripThinking(content string) string {
	content = strings.TrimSpace(content)

	for {
		start := strings.Index(content, "<think>")
		if start == -1 {
			return strings.TrimSpace(content)
		}

		end := strings.Index(content[start:], "</think>")
		if end == -1 {
			return strings.TrimSpace(content[:start])
		}

		end += start + len("</think>")
		content = content[:start] + content[end:]
	}
}

type chatRequest struct {
	Model    string        `json:"model"`
	Messages []chatMessage `json:"messages"`
	Stream   bool          `json:"stream"`
	Options  chatOptions   `json:"options"`
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatOptions struct {
	NumPredict  int     `json:"num_predict"`
	Temperature float64 `json:"temperature"`
}

type chatResponse struct {
	Message chatMessage `json:"message"`
}
