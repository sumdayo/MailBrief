package mailbrief
// GIthubActionsのテスト

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
	// Load .env file if it exists (for local development)
	_ = godotenv.Load()

	// Initialize environment variables
	projectID = os.Getenv("GCP_PROJECT_ID")
	lineChannelAccessToken = os.Getenv("LINE_CHANNEL_ACCESS_TOKEN")
	lineUserID = os.Getenv("LINE_USER_ID")

	// Register the function to handle HTTP requests
	functions.HTTP("ProcessEmails", ProcessEmails)
}

/*
func main() {
	// Check if running in Cloud Functions (FUNCTION_TARGET is set)
	if os.Getenv("FUNCTION_TARGET") != "" {
		// --- Cloud Functions Mode ---
		port := os.Getenv("PORT")
		if port == "" {
			port = "8080"
		}
		if err := funcframework.Start(port); err != nil {
			slog.Error("Failed to start function", "error", err)
			os.Exit(1)
		}
	} else {
		// --- Local Development Mode ---
		// Run periodically every 1 minute
		logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
		logger.Info("=== MailBrief Local Mode Started ===")
		logger.Info("Checking for new emails every 1 minute...")

		// Run immediately once
		runLocalProcess(logger)

		// Then run every 1 minute
		ticker := time.NewTicker(1 * time.Minute)
		defer ticker.Stop()

		for range ticker.C {
			runLocalProcess(logger)
		}
	}
}

// runLocalProcess executes the email processing logic locally
func runLocalProcess(logger *slog.Logger) {
	// Create a mock HTTP request/response for the function
	w := &mockResponseWriter{}
	r, _ := http.NewRequest("GET", "/", nil)

	ProcessEmails(w, r)
}

// mockResponseWriter captures the output for local execution
type mockResponseWriter struct{}

func (m *mockResponseWriter) Header() http.Header         { return http.Header{} }
func (m *mockResponseWriter) Write(b []byte) (int, error) { return len(b), nil }
func (m *mockResponseWriter) WriteHeader(statusCode int)  {}
*/

// ProcessEmails is the Cloud Function entry point
// It checks for unread emails, sends notifications to LINE, and updates the state.
func ProcessEmails(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	// Use TextHandler for human-readable logs
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	// 1. Validate Environment Variables
	if projectID == "" || lineChannelAccessToken == "" || lineUserID == "" {
		logger.Error("❌ Missing required environment variables")
		http.Error(w, "Internal Server Error: Missing configuration", http.StatusInternalServerError)
		return
	}

	// Silenced: logger.Info("🔄 メールチェックを開始します...")

	// 2. Initialize Clients
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

	// 3. Get Last Processed Time
	lastProcessed, err := firestoreClient.GetLastProcessedTime(ctx)
	if err != nil {
		logger.Error("Failed to get last processed time", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// 4. List Unread Messages
	messages, err := gmailClient.ListUnreadMessages(ctx, lastProcessed)
	if err != nil {
		logger.Error("Failed to list messages", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	if len(messages) == 0 {
		// Silenced: logger.Info("✅ 最新メールはありません")
		fmt.Fprint(w, "No new messages")
		return
	}

	// 5. Process Each Message
	processedCount := 0
	var latestTime time.Time = lastProcessed

	for _, msgHeader := range messages {
		// Get full message details
		fullMsg, err := gmailClient.GetMessage(ctx, msgHeader.Id)
		if err != nil {
			logger.Error("Failed to get message details", "id", msgHeader.Id, "error", err)
			continue
		}

		// Determine message time (Convert to UTC for consistent comparison)
		msgTime := time.Unix(fullMsg.InternalDate/1000, 0).UTC()
		lastProcessedUTC := lastProcessed.UTC()

		// Skip messages that have already been processed
		// If msgTime is BEFORE or EQUAL to lastProcessed, skip it.
		if !msgTime.After(lastProcessedUTC) {
			continue
		}

		// Extract Subject and From
		var subject, from string
		for _, h := range fullMsg.Payload.Headers {
			if h.Name == "Subject" {
				subject = h.Value
			}
			if h.Name == "From" {
				from = h.Value
			}
		}

		// Body extraction (simplified)
		body := fullMsg.Snippet

		// Format notification message
		// Display in JST (Local) for readability
		timeStr := msgTime.In(time.Local).Format("2006/01/02 15:04")
		message := fmt.Sprintf("📧 新着メール\n\n受信日時: %s\n差出人: %s\n件名: %s\n\n内容:\n%s", timeStr, from, subject, body)

		// Print to stdout (Log)
		fmt.Println("--------------------------------------------------")
		fmt.Println(message)
		fmt.Println("--------------------------------------------------")

		// Send to LINE
		if err := lineClient.SendMessage(message); err != nil {
			logger.Error("Failed to send notification", "id", msgHeader.Id, "error", err)
			continue
		}

		// Update latest time tracker
		if msgTime.After(latestTime.UTC()) {
			latestTime = msgTime
		}
		processedCount++
	}

	// 7. Update State (only if new messages were processed)
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
