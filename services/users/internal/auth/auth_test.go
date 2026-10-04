package auth

import (
	"testing"
	"time"
)

func TestPasswordHashing(t *testing.T) {
	hash, err := HashPassword("password123")
	if err != nil {
		t.Fatalf("hash failed: %v", err)
	}
	if hash == "password123" {
		t.Fatal("password must not be stored in plain text")
	}
	if !CheckPassword(hash, "password123") {
		t.Error("correct password should match")
	}
	if CheckPassword(hash, "wrong") {
		t.Error("wrong password should not match")
	}
}

func TestTokenRoundTrip(t *testing.T) {
	m := NewManager("secret", time.Hour)
	token, err := m.Issue("user-1")
	if err != nil {
		t.Fatalf("issue failed: %v", err)
	}
	id, err := m.Verify(token)
	if err != nil {
		t.Fatalf("verify failed: %v", err)
	}
	if id != "user-1" {
		t.Errorf("got %q, want user-1", id)
	}
}

func TestTokenWrongSecret(t *testing.T) {
	token, _ := NewManager("secret-a", time.Hour).Issue("user-1")
	if _, err := NewManager("secret-b", time.Hour).Verify(token); err == nil {
		t.Error("token signed with another secret must be rejected")
	}
}

func TestTokenExpired(t *testing.T) {
	m := NewManager("secret", -time.Minute)
	token, _ := m.Issue("user-1")
	if _, err := m.Verify(token); err == nil {
		t.Error("expired token must be rejected")
	}
}