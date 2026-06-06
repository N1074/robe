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

	content := strings.TrimSpace(out.Message.Content)
	if content == "" {
		return "", errors.New("ollama returned empty content")
	}

	return content, nil
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
