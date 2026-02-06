package mailbrief

import (
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/sumdayo/mailbrief/internal/firestore"
	"github.com/sumdayo/mailbrief/internal/gmail"
	"github.com/sumdayo/mailbrief/internal/line"

	"github.com/GoogleCloudPlatform/functions-framework-go/functions"
	"github.com/joho/godotenv"
)

var (
	projectID              string
	lineChannelAccessToken string
	lineUserID             string
)

func init() {
	_ = godotenv.Load()

	projectID = os.Getenv("GCP_PROJECT_ID")
	lineChannelAccessToken = os.Getenv("LINE_CHANNEL_ACCESS_TOKEN")
	lineUserID = os.Getenv("LINE_USER_ID")

	functions.HTTP("ProcessEmails", ProcessEmails)
}

// runLocalProcess executes the email processing logic locally
func runLocalProcess(logger *slog.Logger) {
	w := &mockResponseWriter{}
	r, _ := http.NewRequest("GET", "/", nil)

	ProcessEmails(w, r)
}

type mockResponseWriter struct{}

func (m *mockResponseWriter) Header() http.Header         { return http.Header{} }
func (m *mockResponseWriter) Write(b []byte) (int, error) { return len(b), nil }
func (m *mockResponseWriter) WriteHeader(statusCode int)  {}

// ProcessEmails is the Cloud Function entry point
// It checks for unread emails, sends notifications to LINE, and updates the state.
func ProcessEmails(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	// Use TextHandler for human-readable logs
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	// 各IDが有効であるかチェック
	if projectID == "" || lineChannelAccessToken == "" || lineUserID == "" {
		logger.Error("❌ Missing required environment variables")
		http.Error(w, "Internal Server Error: Missing configuration", http.StatusInternalServerError)
		return
	}

	gmailClient, err := gmail.NewClient(ctx)
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

	messages, err := gmailClient.ListUnreadMessages(ctx, lastProcessed)
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

	// メッセージを受け取る
	for _, msgHeader := range messages {
		fullMsg, err := gmailClient.GetMessage(ctx, msgHeader.Id)
		if err != nil {
			logger.Error("Failed to get message details", "id", msgHeader.Id, "error", err)
			continue
		}

		msgTime := time.Unix(fullMsg.InternalDate/1000, 0).UTC()
		lastProcessedUTC := lastProcessed.UTC()

		if !msgTime.After(lastProcessedUTC) {
			continue
		}

		var subject, from string
		for _, h := range fullMsg.Payload.Headers {
			if h.Name == "Subject" {
				subject = h.Value
			}
			if h.Name == "From" {
				from = h.Value
			}
		}

		body := fullMsg.Snippet

		timeStr := msgTime.In(time.Local).Format("2006/01/02 15:04")
		message := fmt.Sprintf("📧 新着メール\n\n受信日時: %s\n差出人: %s\n件名: %s\n\n内容:\n%s", timeStr, from, subject, body)

		// 【ログ】メールの出力
		fmt.Println("--------------------------------------------------")
		fmt.Println(message)
		fmt.Println("--------------------------------------------------")

		if err := lineClient.SendMessage(message); err != nil {
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
		logger.Info("✅ 処理完了", "processed_count", processedCount)
	} else {
		// Silenced: logger.Info("✅ 新しいメッセージはありませんでした（全て処理済み）")
	}

	fmt.Fprintf(w, "Processed %d messages", processedCount)
}
