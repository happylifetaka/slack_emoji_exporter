package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

const slackEmojiListURL = "https://slack.com/api/emoji.list"

type slackEmojiListResponse struct {
	OK    bool              `json:"ok"`
	Emoji map[string]string `json:"emoji"`
	Error string            `json:"error"`
}

func listEmoji(ctx context.Context, client *http.Client, token string) (map[string]string, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, slackEmojiListURL, nil)
	if err != nil {
		return nil, fmt.Errorf("Slack APIリクエストを作成できません: %w", err)
	}
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("Accept", "application/json")

	response, err := client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("Slack APIへの接続に失敗しました: %w", err)
	}
	defer response.Body.Close()

	if response.StatusCode < 200 || response.StatusCode >= 300 {
		_, _ = io.Copy(io.Discard, response.Body)
		if retryAfter := response.Header.Get("Retry-After"); retryAfter != "" {
			return nil, fmt.Errorf("Slack APIがHTTP %dを返しました（%s秒後に再試行してください）", response.StatusCode, retryAfter)
		}
		return nil, fmt.Errorf("Slack APIがHTTP %dを返しました", response.StatusCode)
	}

	var body slackEmojiListResponse
	decoder := json.NewDecoder(io.LimitReader(response.Body, 10<<20))
	if err := decoder.Decode(&body); err != nil {
		return nil, fmt.Errorf("Slack APIから不正なJSONレスポンスを受信しました: %w", err)
	}
	if !body.OK {
		if body.Error == "" {
			body.Error = "unknown_error"
		}
		return nil, fmt.Errorf("Slack APIエラー: %s", body.Error)
	}
	if body.Emoji == nil {
		return nil, fmt.Errorf("Slack APIレスポンスにemojiがありません")
	}
	return body.Emoji, nil
}
