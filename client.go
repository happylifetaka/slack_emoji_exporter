package main

import (
	"net/http"
	"time"
)

// newHTTPClientはSlack APIと絵文字画像を取得するクライアントを作る。
// このアプリ自身がHTTPサーバーとして待ち受けることはない。
func newHTTPClient(timeout time.Duration) *http.Client {
	return &http.Client{
		Timeout: timeout,
		CheckRedirect: func(request *http.Request, via []*http.Request) error {
			if len(via) >= 10 {
				return http.ErrUseLastResponse
			}
			return nil
		},
	}
}
