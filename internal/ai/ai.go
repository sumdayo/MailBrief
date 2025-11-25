package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"cloud.google.com/go/vertexai/genai"
)

type Client struct {
	client *genai.Client
	model  *genai.GenerativeModel
}

type AnalysisResult struct {
	Summary    string `json:"summary"`
	ReplyDraft string `json:"reply_draft"`
	Necessity  string `json:"necessity"`
}

func NewClient(ctx context.Context, projectID string, location string) (*Client, error) {
	client, err := genai.NewClient(ctx, projectID, location)
	if err != nil {
		return nil, fmt.Errorf("failed to create vertex ai client: %v", err)
	}
	model := client.GenerativeModel("gemini-2.0-flash-exp")
	model.ResponseMIMEType = "application/json"
	return &Client{client: client, model: model}, nil
}

func (c *Client) Close() error {
	return c.client.Close()
}

func (c *Client) AnalyzeEmail(ctx context.Context, subject, body string) (*AnalysisResult, error) {
	prompt := fmt.Sprintf(`
You are a helpful assistant for a software engineer.
Please analyze the following email and provide a summary and a reply draft.

User Profile:
- Role: Software Engineer
- Tone: Polite but concise
- Attitude: Positive towards all emails

Email Subject: %s
Email Body:
%s

Output JSON format:
{
  "summary": "3 line summary of the email",
  "reply_draft": "Draft reply based on the user profile",
  "necessity": "High/Medium/Low"
}
`, subject, body)

	resp, err := c.model.GenerateContent(ctx, genai.Text(prompt))
	if err != nil {
		return nil, fmt.Errorf("failed to generate content: %v", err)
	}

	if len(resp.Candidates) == 0 || len(resp.Candidates[0].Content.Parts) == 0 {
		return nil, fmt.Errorf("no content generated")
	}

	part := resp.Candidates[0].Content.Parts[0]
	text, ok := part.(genai.Text)
	if !ok {
		return nil, fmt.Errorf("generated content is not text")
	}

	var result AnalysisResult
	// Clean up markdown code blocks if present
	jsonStr := string(text)
	jsonStr = strings.TrimPrefix(jsonStr, "```json")
	jsonStr = strings.TrimPrefix(jsonStr, "```")
	jsonStr = strings.TrimSuffix(jsonStr, "```")
	jsonStr = strings.TrimSpace(jsonStr)

	// Gemini 2.0 might return an array, handle both cases
	if strings.HasPrefix(jsonStr, "[") {
		var results []AnalysisResult
		if err := json.Unmarshal([]byte(jsonStr), &results); err != nil {
			return nil, fmt.Errorf("failed to unmarshal json array: %v, text: %s", err, jsonStr)
		}
		if len(results) == 0 {
			return nil, fmt.Errorf("no analysis results returned")
		}
		return &results[0], nil
	}

	if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal json: %v, text: %s", err, jsonStr)
	}

	return &result, nil
}
