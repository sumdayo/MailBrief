package mailbrief

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/sumdayo/mailbrief/internal/firestore"
	"github.com/sumdayo/mailbrief/internal/gmail"
	"github.com/sumdayo/mailbrief/internal/line"

	"github.com/GoogleCloudPlatform/functions-framework-go/functions"
	cloudevents "github.com/cloudevents/sdk-go/v2"
	"github.com/joho/godotenv"
	gmailapi "google.golang.org/api/gmail/v1"
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
	functions.CloudEvent("ProcessPubSubEvent", ProcessPubSubEvent)
}

func runLocalProcess(logger *slog.Logger) {
	w := &mockResponseWriter{}
	r, _ := http.NewRequest("GET", "/", nil)

	ProcessEmails(w, r)
}

type mockResponseWriter struct{}

func (m *mockResponseWriter) Header() http.Header         { return http.Header{} }
func (m *mockResponseWriter) Write(b []byte) (int, error) { return len(b), nil }
func (m *mockResponseWriter) WriteHeader(statusCode int)  {}

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
		http.Error(w, "内部のサーバーエラ－", http.StatusInternalServerError)
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

type pubSubMessage struct {
	Data       string            `json:"data"`
	Attributes map[string]string `json:"attributes"`
}

type pubSubEvent struct {
	Message pubSubMessage `json:"message"`
}

type gmailPushPayload struct {
	EmailAddress string          `json:"emailAddress"`
	HistoryID    json.RawMessage `json:"historyId"`
}

// ProcessPubSubEvent handles Gmail push notifications delivered via Pub/Sub.
func ProcessPubSubEvent(ctx context.Context, event cloudevents.Event) error {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	if projectID == "" || lineChannelAccessToken == "" || lineUserID == "" {
		logger.Error("❌ Missing required environment variables")
		return fmt.Errorf("missing configuration")
	}

	var ps pubSubEvent
	if err := event.DataAs(&ps); err != nil {
		logger.Error("Failed to parse CloudEvent data", "error", err)
		return err
	}

	if ps.Message.Data == "" {
		logger.Error("Pub/Sub message data is empty")
		return fmt.Errorf("pubsub message data is empty")
	}

	payload, err := decodeGmailPushPayload(ps.Message.Data)
	if err != nil {
		logger.Error("Failed to decode Pub/Sub payload", "error", err)
		return err
	}

	historyID, err := parseHistoryID(payload.HistoryID)
	if err != nil {
		logger.Error("Invalid historyId in payload", "error", err)
		return err
	}

	gmailClient, err := gmail.NewClient(ctx)
	if err != nil {
		logger.Error("Failed to create Gmail client", "error", err)
		return err
	}

	firestoreClient, err := firestore.NewClient(ctx, projectID)
	if err != nil {
		logger.Error("Failed to create Firestore client", "error", err)
		return err
	}
	defer firestoreClient.Close()

	lineClient, err := line.NewClient(lineChannelAccessToken, lineUserID)
	if err != nil {
		logger.Error("Failed to create LINE client", "error", err)
		return err
	}

	lastHistoryID, err := firestoreClient.GetLastHistoryID(ctx)
	if err != nil {
		logger.Error("Failed to get last history id", "error", err)
		return err
	}

	var messages []*gmailapi.Message
	if lastHistoryID == 0 {
		lastProcessed, err := firestoreClient.GetLastProcessedTime(ctx)
		if err != nil {
			logger.Error("Failed to get last processed time", "error", err)
			return err
		}
		messages, err = gmailClient.ListUnreadMessages(ctx, lastProcessed)
		if err != nil {
			logger.Error("Failed to list messages", "error", err)
			return err
		}
	} else {
		messages, err = gmailClient.ListHistoryMessages(ctx, lastHistoryID)
		if err != nil {
			logger.Error("Failed to list history messages", "error", err)
			return err
		}
	}

	if len(messages) == 0 {
		if err := firestoreClient.UpdateLastHistoryID(ctx, historyID); err != nil {
			logger.Error("Failed to update history id", "error", err)
		}
		return nil
	}

	processedCount := 0
	var latestTime time.Time

	for _, msgHeader := range messages {
		fullMsg, err := gmailClient.GetMessage(ctx, msgHeader.Id)
		if err != nil {
			logger.Error("Failed to get message details", "id", msgHeader.Id, "error", err)
			continue
		}

		msgTime := time.Unix(fullMsg.InternalDate/1000, 0).UTC()
		if msgTime.After(latestTime.UTC()) {
			latestTime = msgTime
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

		if err := lineClient.SendMessage(message); err != nil {
			logger.Error("Failed to send notification", "id", msgHeader.Id, "error", err)
			continue
		}

		processedCount++
	}

	if processedCount > 0 {
		if !latestTime.IsZero() {
			if err := firestoreClient.UpdateLastProcessedTime(ctx, latestTime); err != nil {
				logger.Error("Failed to update state", "error", err)
			}
		}
		if err := firestoreClient.UpdateLastHistoryID(ctx, historyID); err != nil {
			logger.Error("Failed to update history id", "error", err)
		}
		logger.Info("✅ 処理完了", "processed_count", processedCount)
	}

	return nil
}

func decodeGmailPushPayload(b64 string) (*gmailPushPayload, error) {
	data, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return nil, fmt.Errorf("failed to base64 decode: %v", err)
	}
	var payload gmailPushPayload
	if err := json.Unmarshal(data, &payload); err != nil {
		return nil, fmt.Errorf("failed to unmarshal payload: %v", err)
	}
	return &payload, nil
}

func parseHistoryID(raw json.RawMessage) (uint64, error) {
	if len(raw) == 0 {
		return 0, fmt.Errorf("historyId is empty")
	}

	var asString string
	if err := json.Unmarshal(raw, &asString); err == nil {
		asString = strings.TrimSpace(asString)
		if asString == "" {
			return 0, fmt.Errorf("historyId is empty")
		}
		return strconv.ParseUint(asString, 10, 64)
	}

	var asNumber json.Number
	if err := json.Unmarshal(raw, &asNumber); err == nil {
		return strconv.ParseUint(asNumber.String(), 10, 64)
	}

	return 0, fmt.Errorf("historyId has unsupported type")
}
