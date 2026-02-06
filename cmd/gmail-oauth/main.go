package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"strconv"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/gmail/v1"
)

func main() {
	credsPath := os.Getenv("GMAIL_OAUTH_CLIENT_FILE")
	if credsPath == "" {
		credsPath = "credentials.json"
	}

	b, err := os.ReadFile(credsPath)
	if err != nil {
		exitf("failed to read credentials file: %v", err)
	}

	config, err := google.ConfigFromJSON(b, gmail.GmailReadonlyScope)
	if err != nil {
		exitf("failed to parse credentials: %v", err)
	}

	tok, err := getTokenFromWeb(config)
	if err != nil {
		exitf("failed to get token from web: %v", err)
	}

	if err := saveAuthorizedUserJSON("token.json", config, tok); err != nil {
		exitf("failed to save token: %v", err)
	}

	fmt.Println("Saved authorized user credentials to token.json")
	fmt.Println("Set GMAIL_OAUTH_TOKEN_JSON to the contents of token.json.")
}

func getTokenFromWeb(config *oauth2.Config) (*oauth2.Token, error) {
	port := envInt("OAUTH_REDIRECT_PORT", 8085)
	redirectURL := fmt.Sprintf("http://localhost:%d", port)
	config.RedirectURL = redirectURL

	codeCh := make(chan string, 1)
	errCh := make(chan error, 1)

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("error") != "" {
			errCh <- fmt.Errorf("oauth error: %s", r.URL.Query().Get("error"))
			http.Error(w, "OAuth error. You can close this window.", http.StatusBadRequest)
			return
		}
		code := r.URL.Query().Get("code")
		if code == "" {
			http.Error(w, "No code found. You can close this window.", http.StatusBadRequest)
			return
		}
		fmt.Fprintln(w, "Authorization complete. You can close this window.")
		codeCh <- code
	})

	ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		return nil, fmt.Errorf("failed to listen on %s: %v", redirectURL, err)
	}
	srv := &http.Server{Handler: mux}
	defer func() {
		_ = srv.Close()
	}()

	go func() {
		if err := srv.Serve(ln); err != nil && !errorsIsServerClosed(err) {
			errCh <- err
		}
	}()

	authURL := config.AuthCodeURL("state-token", oauth2.AccessTypeOffline, oauth2.ApprovalForce)
	fmt.Printf("Open this URL in your browser:\n%v\n", authURL)
	fmt.Printf("Waiting for OAuth redirect on %s ...\n", redirectURL)

	select {
	case code := <-codeCh:
		return config.Exchange(context.Background(), code)
	case err := <-errCh:
		return nil, err
	case <-time.After(2 * time.Minute):
		return nil, fmt.Errorf("timed out waiting for OAuth redirect")
	}
}

func saveAuthorizedUserJSON(path string, config *oauth2.Config, token *oauth2.Token) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	payload := map[string]string{
		"type":          "authorized_user",
		"client_id":     config.ClientID,
		"client_secret": config.ClientSecret,
		"refresh_token": token.RefreshToken,
	}
	return json.NewEncoder(f).Encode(payload)
}

func exitf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}

func envInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}

func errorsIsServerClosed(err error) bool {
	return err == http.ErrServerClosed
}
