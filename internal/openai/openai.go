package openai

import (
	"context"
	"fmt"
	"os"

	"github.com/sashabaranov/go-openai"
)

// GenerateReply generates a reply to an email using OpenAI's ChatCompletion API.
func GenerateReply(ctx context.Context, mailBody string) (string, error) {
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		return "", fmt.Errorf("OPENAI_API_KEY is not set")
	}

	client := openai.NewClient(apiKey)

	prompt := fmt.Sprintf("以下のメールに対して、プロフェッショナルかつ丁寧な返信を日本語で生成してください。返信内容のみを生成し、署名や件名は含めないでください。\n\n--- メール本文 ---\n%s", mailBody)

	resp, err := client.CreateChatCompletion(
		ctx,
		openai.ChatCompletionRequest{
			Model: openai.GPT4o,
			Messages: []openai.ChatCompletionMessage{
				{
					Role:    openai.ChatMessageRoleSystem,
					Content: "あなたは、受け取ったメールに対して、プロフェッショナルで丁寧な返信メールを作成するアシスタントです。",
				},
				{
					Role:    openai.ChatMessageRoleUser,
					Content: prompt,
				},
			},
		},
	)

	if err != nil {
		return "", fmt.Errorf("failed to create chat completion: %w", err)
	}

	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("no choices returned from OpenAI")
	}

	return resp.Choices[0].Message.Content, nil
}
