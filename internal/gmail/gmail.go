package gmail

import (
	"context"
	"fmt"
	"os"
	"strings" 
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

// フィルタリング機能
func (c *Client) ListUnreadMessages(ctx context.Context, after time.Time) ([]*gmail.Message, error) {
	user := os.Getenv("GMAIL_USER")
	if user == "" {
		user = "me"
	}

	dateStr := after.Format("2006/01/02")
	queryParts := []string{
		"is:unread",
		fmt.Sprintf("after:%s", dateStr),
	}

	// 指定アドレスのみに絞る処理
	targetEmails := os.Getenv("TARGET_EMAILS")
	
	if targetEmails != "" {
		emails := strings.Split(targetEmails, ",")

		var fromQueries []string
		for _, email := range emails {
			fromQueries = append(fromQueries, fmt.Sprintf("from:%s", strings.TrimSpace(email)))
		}
		
		if len(fromQueries) > 0 {
			queryParts = append(queryParts, fmt.Sprintf("(%s)", strings.Join(fromQueries, " OR ")))
		}
	}

	finalQuery := strings.Join(queryParts, " ")
	
	var messages []*gmail.Message
	pageToken := ""
	for {
		req := c.service.Users.Messages.List(user).Q(finalQuery).PageToken(pageToken)
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
	user := os.Getenv("GMAIL_USER")
	if user == "" {
		user = "me"
	}
	msg, err := c.service.Users.Messages.Get(user, msgID).Do()
	if err != nil {
		return nil, fmt.Errorf("unable to retrieve message %v: %v", msgID, err)
	}
	return msg, nil
}