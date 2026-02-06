package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/sumdayo/mailbrief/internal/gmail"
)

func main() {
	ctx := context.Background()

	topic := os.Getenv("PUBSUB_TOPIC")
	if topic == "" {
		exitf("PUBSUB_TOPIC is required (format: projects/<PROJECT_ID>/topics/<TOPIC>)")
	}

	var labels []string
	if v := os.Getenv("GMAIL_WATCH_LABELS"); v != "" {
		for _, label := range strings.Split(v, ",") {
			label = strings.TrimSpace(label)
			if label != "" {
				labels = append(labels, label)
			}
		}
	}

	client, err := gmail.NewClient(ctx)
	if err != nil {
		exitf("failed to create gmail client: %v", err)
	}

	resp, err := client.Watch(ctx, topic, labels)
	if err != nil {
		exitf("watch failed: %v", err)
	}

	fmt.Printf("Watch created. historyId=%d, expiration=%d\n", resp.HistoryId, resp.Expiration)
}

func exitf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
