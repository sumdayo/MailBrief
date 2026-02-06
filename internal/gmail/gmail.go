package gmail

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"golang.org/x/oauth2/google"
	"google.golang.org/api/gmail/v1"
	"google.golang.org/api/option"
)

type Client struct {
	service *gmail.Service
}

func NewClient(ctx context.Context) (*Client, error) {
	creds, err := loadOAuthCredentials(ctx)
	if err != nil {
		return nil, fmt.Errorf("unable to load Gmail OAuth credentials: %v", err)
	}

	var srv *gmail.Service
	if creds != nil {
		srv, err = gmail.NewService(ctx, option.WithCredentials(creds))
	} else {
		srv, err = gmail.NewService(ctx, option.WithScopes(gmail.GmailReadonlyScope))
	}
	if err != nil {
		return nil, fmt.Errorf("unable to retrieve Gmail client: %v", err)
	}
	return &Client{service: srv}, nil
}

func loadOAuthCredentials(ctx context.Context) (*google.Credentials, error) {
	if envJSON := os.Getenv("GMAIL_OAUTH_TOKEN_JSON"); envJSON != "" {
		return google.CredentialsFromJSON(ctx, []byte(envJSON), gmail.GmailReadonlyScope)
	}
	if path := os.Getenv("GMAIL_OAUTH_TOKEN_FILE"); path != "" {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("failed to read GMAIL_OAUTH_TOKEN_FILE: %v", err)
		}
		return google.CredentialsFromJSON(ctx, data, gmail.GmailReadonlyScope)
	}
	return nil, nil
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
