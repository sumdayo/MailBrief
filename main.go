package main

import (
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"mailbrief/internal/firestore"
	"mailbrief/internal/gmail"
	"mailbrief/internal/line"

	"github.com/GoogleCloudPlatform/functions-framework-go/funcframework"
	"github.com/GoogleCloudPlatform/functions-framework-go/functions"
	"github.com/joho/godotenv"
)

var (
	projectID              string
	lineChannelAccessToken string
	lineUserID             string
)

func init() {
	// Load .env file if it exists
	_ = godotenv.Load()

	// Initialize environment variables
	projectID = os.Getenv("GCP_PROJECT_ID")
	lineChannelAccessToken = os.Getenv("LINE_CHANNEL_ACCESS_TOKEN")
	lineUserID = os.Getenv("LINE_USER_ID")

	// Register the function to handle HTTP requests
	functions.HTTP("ProcessEmails", ProcessEmails)
}

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	// Ensure FUNCTION_TARGET is set so funcframework knows what to serve
	if os.Getenv("FUNCTION_TARGET") == "" {
		os.Setenv("FUNCTION_TARGET", "ProcessEmails")
	}

	// Use funcframework to start the server for local development
	// Auto-trigger for convenience
	go func() {
		time.Sleep(1 * time.Second)
		resp, err := http.Get(fmt.Sprintf("http://localhost:%s", port))
		if err != nil {
			slog.Error("Failed to auto-trigger", "error", err)
			return
		}
		defer resp.Body.Close()
		slog.Info("Auto-triggered function", "status", resp.Status)
	}()

	if err := funcframework.Start(port); err != nil {
		slog.Error("Failed to start function", "error", err)
		os.Exit(1)
	}
}

// ProcessEmails is the Cloud Function entry point
func ProcessEmails(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	// validate env vars
	if projectID == "" || lineChannelAccessToken == "" || lineUserID == "" {
		logger.Error("Missing required environment variables")
		http.Error(w, "Internal Server Error: Missing configuration", http.StatusInternalServerError)
		return
	}

	logger.Info("Starting email processing")

	// Excluded domains
	excludedDomains := []string{"@careerpark.jp", "@figma.com", "@google.com"}

	// 1. Initialize Clients
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

	// 2. Get Last Processed Time
	lastProcessed, err := firestoreClient.GetLastProcessedTime(ctx)
	if err != nil {
		logger.Error("Failed to get last processed time", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	logger.Info("Last processed time", "time", lastProcessed)

	// 3. List Unread Messages
	messages, err := gmailClient.ListUnreadMessages(ctx, lastProcessed)
	if err != nil {
		logger.Error("Failed to list messages", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	if len(messages) == 0 {
		logger.Info("No new messages found")
		fmt.Fprint(w, "No new messages")
		return
	}

	logger.Info("Found messages", "count", len(messages))

	// 4. Process Each Message
	processedCount := 0
	var latestTime time.Time = lastProcessed

	for _, msgHeader := range messages {
		// Get full message details
		fullMsg, err := gmailClient.GetMessage(ctx, msgHeader.Id)
		if err != nil {
			logger.Error("Failed to get message details", "id", msgHeader.Id, "error", err)
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

		// Check if sender is in excluded domains
		shouldExclude := false
		for _, domain := range excludedDomains {
			if strings.Contains(from, domain) {
				shouldExclude = true
				logger.Info("Skipping excluded domain", "from", from, "domain", domain)
				break
			}
		}

		if shouldExclude {
			continue
		}

		// Body extraction (simplified)
		body := fullMsg.Snippet

		logger.Info("Processing message", "id", msgHeader.Id, "subject", subject, "from", from)

		// 5. Send to LINE directly
		message := fmt.Sprintf("📧 新着メール\n\n差出人: %s\n件名: %s\n\n内容:\n%s", from, subject, body)

		if err := lineClient.SendMessage(message); err != nil {
			logger.Error("Failed to send notification", "id", msgHeader.Id, "error", err)
			continue
		}

		// Update latest time
		msgTime := time.Unix(fullMsg.InternalDate/1000, 0)
		if msgTime.After(latestTime) {
			latestTime = msgTime
		}
		processedCount++
	}

	// 6. Update State
	if processedCount > 0 {
		if err := firestoreClient.UpdateLastProcessedTime(ctx, latestTime); err != nil {
			logger.Error("Failed to update state", "error", err)
		}
	}

	fmt.Fprintf(w, "Processed %d messages", processedCount)
}
