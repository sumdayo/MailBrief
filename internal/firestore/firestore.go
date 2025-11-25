package firestore

import (
	"context"
	"fmt"
	"time"

	"cloud.google.com/go/firestore"
)

type Client struct {
	client *firestore.Client
}

func NewClient(ctx context.Context, projectID string) (*Client, error) {
	client, err := firestore.NewClient(ctx, projectID)
	if err != nil {
		return nil, fmt.Errorf("failed to create firestore client: %v", err)
	}
	return &Client{client: client}, nil
}

func (c *Client) Close() error {
	return c.client.Close()
}

type State struct {
	LastProcessed time.Time `firestore:"last_processed"`
}

func (c *Client) GetLastProcessedTime(ctx context.Context) (time.Time, error) {
	doc, err := c.client.Collection("mail_state").Doc("latest").Get(ctx)
	if err != nil {
		// If document doesn't exist, return a default time (e.g., 24 hours ago)
		if doc == nil || !doc.Exists() {
			return time.Now().Add(-24 * time.Hour), nil
		}
		return time.Time{}, fmt.Errorf("failed to get last processed time: %v", err)
	}
	var state State
	if err := doc.DataTo(&state); err != nil {
		return time.Time{}, fmt.Errorf("failed to parse state: %v", err)
	}
	return state.LastProcessed, nil
}

func (c *Client) UpdateLastProcessedTime(ctx context.Context, t time.Time) error {
	_, err := c.client.Collection("mail_state").Doc("latest").Set(ctx, State{LastProcessed: t})
	if err != nil {
		return fmt.Errorf("failed to update last processed time: %v", err)
	}
	return nil
}
