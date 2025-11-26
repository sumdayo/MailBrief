package line

import (
	"fmt"

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

func (c *Client) SendNotification(summary, replyDraft, necessity string) error {
	message := fmt.Sprintf("📩 New Email Analyzed\n\n【Summary】\n%s\n\n【Necessity】: %s\n\n【Reply Draft】\n%s", summary, necessity, replyDraft)

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
