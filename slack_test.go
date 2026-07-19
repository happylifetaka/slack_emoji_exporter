package main

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (function roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return function(request)
}

func TestListEmoji(t *testing.T) {
	client := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if got := request.Header.Get("Authorization"); got != "Bearer xoxb-secret" {
			t.Fatalf("Authorization = %q", got)
		}
		return jsonResponse(http.StatusOK, `{"ok":true,"emoji":{"wave":"alias:hello"}}`), nil
	})}

	emoji, err := listEmoji(context.Background(), client, "xoxb-secret")
	if err != nil {
		t.Fatalf("listEmoji: %v", err)
	}
	if got := emoji["wave"]; got != "alias:hello" {
		t.Fatalf("wave = %q", got)
	}
}

func TestListEmojiSlackError(t *testing.T) {
	client := &http.Client{Transport: roundTripFunc(func(_ *http.Request) (*http.Response, error) {
		return jsonResponse(http.StatusOK, `{"ok":false,"error":"missing_scope"}`), nil
	})}

	_, err := listEmoji(context.Background(), client, "token")
	if err == nil || !strings.Contains(err.Error(), "missing_scope") {
		t.Fatalf("error = %v", err)
	}
}

func jsonResponse(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}
