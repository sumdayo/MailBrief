package gmail

import (
	"context"
	"fmt"
	"time"

	"google.golang.org/api/gmail/v1"
	"google.golang.org/api/option"
)

type Client struct {
	service *gmail.Service
}

func NewClient(ctx context.Context) (*Client, error) {
	srv, err := gmail.NewService(ctx, option.WithScopes(gmail.GmailReadonlyScope))
	if err != nil {
		return nil, fmt.Errorf("unable to retrieve Gmail client: %v", err)
	}
	return &Client{service: srv}, nil
}

func (c *Client) ListUnreadMessages(ctx context.Context, after time.Time) ([]*gmail.Message, error) {
	user := "me"
	query := fmt.Sprintf("is:unread after:%d", after.Unix())
	
	var messages []*gmail.Message
	pageToken := ""
	for {
		req := c.service.Users.Messages.List(user).Q(query).PageToken(pageToken)
		r, err := req.Do()
		if err != nil {
			return nil, fmt.Errorf("unable to retrieve messages: %v", err)
		}
		messages = append(messages, r.Messages...)
		if r.NextPageToken == "" {
			break
		}
		pageToken = r.NextPageToken
	}
	return messages, nil
}

func (c *Client) GetMessage(ctx context.Context, msgID string) (*gmail.Message, error) {
	user := "me"
	msg, err := c.service.Users.Messages.Get(user, msgID).Do()
	if err != nil {
		return nil, fmt.Errorf("unable to retrieve message %v: %v", msgID, err)
	}
	return msg, nil
}
