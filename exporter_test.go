package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSanitizeFilename(t *testing.T) {
	tests := map[string]string{
		"../../hello world": "hello_world",
		"../..":             "emoji",
		"party-parrot":      "party-parrot",
	}
	for input, want := range tests {
		if got := sanitizeFilename(input); got != want {
			t.Errorf("sanitizeFilename(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestGetExtension(t *testing.T) {
	if got := getExtension("https://example.com/a.png", "image/gif; charset=binary"); got != ".gif" {
		t.Fatalf("Content-Type extension = %q", got)
	}
	if got := getExtension("https://example.com/a.webp?x=1", ""); got != ".webp" {
		t.Fatalf("URL extension = %q", got)
	}
}

func TestExportEmoji(t *testing.T) {
	client := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.URL.Path == "/ok.png" {
			return imageResponse(http.StatusOK, "image/png", []byte{1, 2, 3}), nil
		}
		return imageResponse(http.StatusNotFound, "text/plain", []byte("not found")), nil
	})}

	outputDir := t.TempDir()
	manifest, err := exportEmoji(context.Background(), client, map[string]string{
		"good":       "https://example.com/ok.png",
		"alias_good": "alias:good",
		"broken":     "https://example.com/missing.png",
	}, exportOptions{outputDir: outputDir, concurrency: 2})
	if err != nil {
		t.Fatalf("exportEmoji: %v", err)
	}
	if manifest.Total != 3 || manifest.Downloaded != 1 || manifest.Aliases != 1 || manifest.Failed != 1 {
		t.Fatalf("manifest = %+v", manifest)
	}

	data, err := os.ReadFile(filepath.Join(outputDir, "emoji", "good.png"))
	if err != nil {
		t.Fatalf("read emoji: %v", err)
	}
	if string(data) != string([]byte{1, 2, 3}) {
		t.Fatalf("emoji data = %v", data)
	}

	manifestData, err := os.ReadFile(filepath.Join(outputDir, "result.json"))
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	var saved exportManifest
	if err := json.Unmarshal(manifestData, &saved); err != nil {
		t.Fatalf("decode manifest: %v", err)
	}
	if saved.Total != 3 {
		t.Fatalf("saved total = %d", saved.Total)
	}

}

func TestExportEmojiSkipsExistingFile(t *testing.T) {
	client := &http.Client{Transport: roundTripFunc(func(_ *http.Request) (*http.Response, error) {
		return imageResponse(http.StatusOK, "image/png", []byte{9}), nil
	})}

	outputDir := t.TempDir()
	input := map[string]string{"good": "https://example.com/good.png"}
	options := exportOptions{outputDir: outputDir, concurrency: 1}
	if _, err := exportEmoji(context.Background(), client, input, options); err != nil {
		t.Fatalf("first export: %v", err)
	}
	manifest, err := exportEmoji(context.Background(), client, input, options)
	if err != nil {
		t.Fatalf("second export: %v", err)
	}
	if manifest.Skipped != 1 {
		t.Fatalf("skipped = %d", manifest.Skipped)
	}
}

func TestExportEmojiRejectsImageOverLimit(t *testing.T) {
	client := &http.Client{Transport: roundTripFunc(func(_ *http.Request) (*http.Response, error) {
		return imageResponse(
			http.StatusOK,
			"image/png",
			bytes.Repeat([]byte{1}, int(maxEmojiBytes+1)),
		), nil
	})}

	outputDir := t.TempDir()
	manifest, err := exportEmoji(context.Background(), client, map[string]string{
		"too_large": "https://example.com/too-large.png",
	}, exportOptions{outputDir: outputDir, concurrency: 1})
	if err != nil {
		t.Fatalf("exportEmoji: %v", err)
	}
	if manifest.Failed != 1 || !strings.Contains(manifest.Emoji[0].Error, "1 MiB") {
		t.Fatalf("manifest = %+v", manifest)
	}
	if _, err := os.Stat(filepath.Join(outputDir, "emoji", "too_large.png")); !os.IsNotExist(err) {
		t.Fatalf("oversized file was not removed: %v", err)
	}
}

func imageResponse(status int, contentType string, body []byte) *http.Response {
	return &http.Response{
		StatusCode: status,
		Header:     http.Header{"Content-Type": []string{contentType}},
		Body:       io.NopCloser(strings.NewReader(string(body))),
	}
}
