package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/joho/godotenv"
	"github.com/line/line-bot-sdk-go/v8/linebot/webhook"
	"google.golang.org/api/gmail/v1"

	"github.com/sumdayo/mailbrief/internal/firestore"
	gmailClient "github.com/sumdayo/mailbrief/internal/gmail"
	"github.com/sumdayo/mailbrief/internal/line"
	"github.com/sumdayo/mailbrief/internal/openai"
)

var (
	projectID              string
	lineChannelAccessToken string
	lineChannelSecret      string
	lineUserID             string
)

func init() {
	_ = godotenv.Load()
	projectID = os.Getenv("GCP_PROJECT_ID")
	lineChannelAccessToken = os.Getenv("LINE_CHANNEL_ACCESS_TOKEN")
	lineChannelSecret = os.Getenv("LINE_CHANNEL_SECRET")
	lineUserID = os.Getenv("LINE_USER_ID")
}

func main() {
	port := "8080"
	slog.Info("=== MailBrief Local Server (net/http) Started ===", "port", port)
	http.HandleFunc("/HandleWebhook", HandleWebhook)
	http.HandleFunc("/ProcessEmails", ProcessEmails)
	slog.Info("Listening for requests on http://localhost:8080")
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		slog.Error("Failed to start http server", "error", err)
		os.Exit(1)
	}
}

// HandleWebhook handles LINE Webhook requests.
func HandleWebhook(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		slog.Error("Failed to read request body", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	r.Body = io.NopCloser(bytes.NewBuffer(body))

	ctx := r.Context()
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	lineClient, err := line.NewClient(lineChannelAccessToken, lineUserID)
	if err != nil {
		logger.Error("Failed to create LINE client", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	cb, err := webhook.ParseRequest(lineChannelSecret, r)
	if err != nil {
		logger.Error("Failed to parse webhook request", "error", err)
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	for _, event := range cb.Events {
		// The LINE SDK for Go v8 returns event objects as values, not pointers.
		// We need to match the case accordingly.
		switch e := event.(type) {
		case webhook.PostbackEvent:
			handlePostbackEvent(ctx, logger, lineClient, &e) // Pass as a pointer
		default:
			// Ignore other event types
		}
	}

	fmt.Fprint(w, "OK")
}

func handlePostbackEvent(ctx context.Context, logger *slog.Logger, lineClient *line.Client, e *webhook.PostbackEvent) {
	logger.Info("Received postback event", "data", e.Postback.Data)

	data, err := url.ParseQuery(e.Postback.Data)
	if err != nil {
		logger.Error("Failed to parse postback data", "error", err)
		return
	}

	action := data.Get("action")
	messageId := data.Get("messageId")

	if action == "create_reply" && messageId != "" {
		go func() {
			if err := processAIReply(context.Background(), logger, lineClient, messageId); err != nil {
				logger.Error("Failed to process AI reply", "error", err)
				lineClient.SendMessage(fmt.Sprintf("エラーが発生しました: %v", err))
			}
		}()
	}
}

func processAIReply(ctx context.Context, logger *slog.Logger, lineClient *line.Client, messageId string) error {
	lineClient.SendMessage("🤖 AIによる返信文の作成を開始します...")

	client, err := gmailClient.NewClient(ctx)
	if err != nil {
		return fmt.Errorf("failed to create gmail client: %w", err)
	}

	msg, err := client.GetMessage(ctx, messageId)
	if err != nil {
		return fmt.Errorf("failed to get original message: %w", err)
	}

	subject := getHeaderValue(msg.Payload.Headers, "Subject")
	from := getHeaderValue(msg.Payload.Headers, "From")
	inReplyTo := getHeaderValue(msg.Payload.Headers, "Message-ID")
	references := getHeaderValue(msg.Payload.Headers, "References")
	if references != "" {
		references += " " + inReplyTo
	} else {
		references = inReplyTo
	}

	replyBody, err := openai.GenerateReply(ctx, msg.Snippet)
	if err != nil {
		return fmt.Errorf("failed to generate reply: %w", err)
	}

	replySubject := "Re: " + subject
	err = client.SendReply(ctx, msg.ThreadId, from, replySubject, replyBody, inReplyTo, references)
	if err != nil {
		return fmt.Errorf("failed to send reply: %w", err)
	}

	successMsg := fmt.Sprintf("✅ 以下の内容で返信を送信しました:\n\nTo: %s\n件名: %s\n\n%s", from, replySubject, replyBody)
	lineClient.SendMessage(successMsg)
	logger.Info("Successfully sent an AI-generated reply", "messageId", messageId)

	return nil
}

// ProcessEmails checks for unread emails and sends notifications.
func ProcessEmails(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	if projectID == "" || lineChannelAccessToken == "" || lineUserID == "" {
		logger.Error("❌ Missing required environment variables")
		http.Error(w, "Internal Server Error: Missing configuration", http.StatusInternalServerError)
		return
	}

	client, err := gmailClient.NewClient(ctx)
	if err != nil {
		logger.Error("Failed to create Gmail client", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	firestoreClient, err := firestore.NewClient(ctx, projectID)
	if err != nil {
		logger.Error("Failed to create Firestore client", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	defer firestoreClient.Close()

	lineClient, err := line.NewClient(lineChannelAccessToken, lineUserID)
	if err != nil {
		logger.Error("Failed to create LINE client", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	lastProcessed, err := firestoreClient.GetLastProcessedTime(ctx)
	if err != nil {
		logger.Error("Failed to get last processed time", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	messages, err := client.ListUnreadMessages(ctx, lastProcessed)
	if err != nil {
		logger.Error("Failed to list messages", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	if len(messages) == 0 {
		fmt.Fprint(w, "No new messages")
		return
	}

	processedCount := 0
	var latestTime time.Time = lastProcessed

	for _, msgHeader := range messages {
		fullMsg, err := client.GetMessage(ctx, msgHeader.Id)
		if err != nil {
			logger.Error("Failed to get message details", "id", msgHeader.Id, "error", err)
			continue
		}

		msgTime := time.Unix(fullMsg.InternalDate/1000, 0).UTC()
		if !msgTime.After(lastProcessed.UTC()) {
			continue
		}

		subject := getHeaderValue(fullMsg.Payload.Headers, "Subject")
		from := getHeaderValue(fullMsg.Payload.Headers, "From")
		snippet := fullMsg.Snippet

		if err := lineClient.SendNewEmailNotification(subject, from, snippet, fullMsg.Id); err != nil {
			logger.Error("Failed to send notification", "id", msgHeader.Id, "error", err)
			continue
		}

		if msgTime.After(latestTime.UTC()) {
			latestTime = msgTime
		}
		processedCount++
	}

	if processedCount > 0 {
		if err := firestoreClient.UpdateLastProcessedTime(ctx, latestTime); err != nil {
			logger.Error("Failed to update state", "error", err)
		}
		logger.Info("✅ Processed new emails", "count", processedCount)
	}

	fmt.Fprintf(w, "Processed %d messages", processedCount)
}

// getHeaderValue is a helper to extract a specific header from a message.
func getHeaderValue(headers []*gmail.MessagePartHeader, name string) string {
	for _, h := range headers {
		if h.Name == name {
			return h.Value
		}
	}
	return ""
}
