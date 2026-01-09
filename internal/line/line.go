package line

import (
	"fmt"
	"unicode/utf8"

	"github.com/line/line-bot-sdk-go/v8/linebot/messaging_api"
)

type Client struct {
	bot    *messaging_api.MessagingApiAPI
	userID string
}

func NewClient(channelAccessToken, userID string) (*Client, error) {
	if channelAccessToken == "" {
		return nil, fmt.Errorf("channel access token is required")
	}
	if userID == "" {
		return nil, fmt.Errorf("user ID is required")
	}

	bot, err := messaging_api.NewMessagingApiAPI(channelAccessToken)
	if err != nil {
		return nil, fmt.Errorf("failed to create LINE bot client: %v", err)
	}
	return &Client{bot: bot, userID: userID}, nil
}

// SendNewEmailNotification sends a notification with a reply button for a new email.
func (c *Client) SendNewEmailNotification(subject, from, snippet, messageId string) error {
	text := fmt.Sprintf("差出人: %s\n件名: %s\n\n%s", from, subject, snippet)
	// Use RuneCountInString for accurate character count and truncate by runes
	if utf8.RuneCountInString(text) > 60 {
		runes := []rune(text)
		text = string(runes[:57]) + "..."
	}

	template := messaging_api.ButtonsTemplate{
		Title: "新着メール",
		Text:  text,
		Actions: []messaging_api.ActionInterface{
			&messaging_api.PostbackAction{
				Label: "AIで返信を作成",
				Data:  fmt.Sprintf("action=create_reply&messageId=%s", messageId),
			},
		},
	}

	_, err := c.bot.PushMessage(
		&messaging_api.PushMessageRequest{
			To: c.userID,
			Messages: []messaging_api.MessageInterface{
				&messaging_api.TemplateMessage{
					AltText:  "新着メールがあります。",
					Template: &template,
				},
			},
		},
		"", // xLineRetryKey
	)
	if err != nil {
		return fmt.Errorf("failed to send notification: %w", err)
	}
	return nil
}

// SendMessage sends a simple text message to the user.
func (c *Client) SendMessage(message string) error {
	_, err := c.bot.PushMessage(
		&messaging_api.PushMessageRequest{
			To: c.userID,
			Messages: []messaging_api.MessageInterface{
				&messaging_api.TextMessage{
					Text: message,
				},
			},
		},
		"", // xLineRetryKey
	)
	if err != nil {
		return fmt.Errorf("failed to send line message: %v", err)
	}
	return nil
}
