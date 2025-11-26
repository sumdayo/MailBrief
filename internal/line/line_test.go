package line

import (
	"testing"
)

func TestNewClient(t *testing.T) {
	// 正常系: トークンとユーザーIDがある場合
	client, err := NewClient("test-token", "test-user-id")
	if err != nil {
		t.Errorf("NewClient returned error: %v", err)
	}
	if client == nil {
		t.Error("NewClient returned nil client")
	}

	// 異常系: トークンがない場合
	_, err = NewClient("", "test-user-id")
	if err == nil {
		t.Error("NewClient should return error when token is empty")
	}

	// 異常系: ユーザーIDがない場合
	_, err = NewClient("test-token", "")
	if err == nil {
		t.Error("NewClient should return error when userID is empty")
	}
}
