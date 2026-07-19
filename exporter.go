package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// Slackはカスタム絵文字に128 KB以下の画像を推奨している。
// 既存・旧仕様の絵文字にも余裕を持たせ、ダウンロード時の安全上限は1 MiBとする。
// https://slack.com/intl/ja-jp/help/articles/206870177
const maxEmojiBytes int64 = 1 << 20

type exportStatus string

const (
	statusDownloaded exportStatus = "downloaded"
	statusSkipped    exportStatus = "skipped"
	statusFailed     exportStatus = "failed"
	statusAlias      exportStatus = "alias"
)

type exportedEmoji struct {
	Name     string       `json:"name"`
	Status   exportStatus `json:"status"`
	Source   string       `json:"source"`
	File     string       `json:"file,omitempty"`
	AliasFor string       `json:"aliasFor,omitempty"`
	Error    string       `json:"error,omitempty"`
}

type exportManifest struct {
	ExportedAt string          `json:"exportedAt"`
	Total      int             `json:"total"`
	Downloaded int             `json:"downloaded"`
	Skipped    int             `json:"skipped"`
	Failed     int             `json:"failed"`
	Aliases    int             `json:"aliases"`
	Emoji      []exportedEmoji `json:"emoji"`
}

type exportOptions struct {
	outputDir   string
	concurrency int
	overwrite   bool
	onProgress  func(completed, total int, item exportedEmoji)
}

type emojiEntry struct {
	name   string
	source string
}

func exportEmoji(ctx context.Context, client *http.Client, emojiMap map[string]string, options exportOptions) (exportManifest, error) {
	imageDir := filepath.Join(options.outputDir, "emoji")
	if err := os.MkdirAll(imageDir, 0o755); err != nil {
		return exportManifest{}, fmt.Errorf("出力ディレクトリを作成できません: %w", err)
	}

	entries := sortedEntries(emojiMap)
	results := make([]exportedEmoji, len(entries))
	jobs := make(chan int)
	var completed atomic.Int64
	var progressMu sync.Mutex
	var workers sync.WaitGroup

	workerCount := min(options.concurrency, len(entries))
	for range workerCount {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for index := range jobs {
				entry := entries[index]
				var result exportedEmoji
				if strings.HasPrefix(entry.source, "alias:") {
					result = exportedEmoji{
						Name: entry.name, Status: statusAlias, Source: entry.source,
						AliasFor: strings.TrimPrefix(entry.source, "alias:"),
					}
				} else {
					result = downloadEmoji(ctx, client, entry, imageDir, options.outputDir, options.overwrite)
				}
				results[index] = result
				current := int(completed.Add(1))
				if options.onProgress != nil {
					progressMu.Lock()
					options.onProgress(current, len(entries), result)
					progressMu.Unlock()
				}
			}
		}()
	}

	for index := range entries {
		jobs <- index
	}
	close(jobs)
	workers.Wait()

	manifest := buildManifest(results)
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return exportManifest{}, fmt.Errorf("manifestを生成できません: %w", err)
	}
	data = append(data, '\n')
	if err := os.WriteFile(filepath.Join(options.outputDir, "result.json"), data, 0o644); err != nil {
		return exportManifest{}, fmt.Errorf("result.jsonを書き込めません: %w", err)
	}
	return manifest, nil
}

func downloadEmoji(ctx context.Context, client *http.Client, entry emojiEntry, imageDir, outputDir string, overwrite bool) exportedEmoji {
	result := exportedEmoji{Name: entry.name, Source: entry.source}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, entry.source, nil)
	if err != nil {
		return failedResult(result, err)
	}
	response, err := client.Do(request)
	if err != nil {
		return failedResult(result, err)
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		_, _ = io.Copy(io.Discard, response.Body)
		return failedResult(result, fmt.Errorf("HTTP %d", response.StatusCode))
	}

	extension := getExtension(entry.source, response.Header.Get("Content-Type"))
	absolutePath := filepath.Join(imageDir, sanitizeFilename(entry.name)+extension)
	relativePath, err := filepath.Rel(outputDir, absolutePath)
	if err != nil {
		return failedResult(result, err)
	}
	result.File = filepath.ToSlash(relativePath)

	flags := os.O_WRONLY | os.O_CREATE
	if overwrite {
		flags |= os.O_TRUNC
	} else {
		flags |= os.O_EXCL
	}
	file, err := os.OpenFile(absolutePath, flags, 0o644)
	if err != nil {
		if !overwrite && os.IsExist(err) {
			result.Status = statusSkipped
			return result
		}
		return failedResult(result, err)
	}

	written, copyErr := io.Copy(file, io.LimitReader(response.Body, maxEmojiBytes+1))
	closeErr := file.Close()
	if copyErr != nil || closeErr != nil || written > maxEmojiBytes {
		_ = os.Remove(absolutePath)
		if copyErr != nil {
			return failedResult(result, copyErr)
		}
		if closeErr != nil {
			return failedResult(result, closeErr)
		}
		return failedResult(result, fmt.Errorf("画像が上限の%d MiBを超えています", maxEmojiBytes>>20))
	}
	result.Status = statusDownloaded
	return result
}

func buildManifest(results []exportedEmoji) exportManifest {
	manifest := exportManifest{
		ExportedAt: time.Now().UTC().Format(time.RFC3339),
		Total:      len(results),
		Emoji:      results,
	}
	for _, result := range results {
		switch result.Status {
		case statusDownloaded:
			manifest.Downloaded++
		case statusSkipped:
			manifest.Skipped++
		case statusFailed:
			manifest.Failed++
		case statusAlias:
			manifest.Aliases++
		}
	}
	return manifest
}

func failedResult(result exportedEmoji, err error) exportedEmoji {
	result.Status = statusFailed
	result.Error = err.Error()
	return result
}

func sortedEntries(emojiMap map[string]string) []emojiEntry {
	names := make([]string, 0, len(emojiMap))
	for name := range emojiMap {
		names = append(names, name)
	}
	sort.Strings(names)
	entries := make([]emojiEntry, 0, len(names))
	for _, name := range names {
		entries = append(entries, emojiEntry{name: name, source: emojiMap[name]})
	}
	return entries
}

var (
	unsafeFilenameCharacters = regexp.MustCompile(`[^a-zA-Z0-9_-]`)
	repeatedUnderscores      = regexp.MustCompile(`_+`)
)

func sanitizeFilename(name string) string {
	name = unsafeFilenameCharacters.ReplaceAllString(name, "_")
	name = repeatedUnderscores.ReplaceAllString(name, "_")
	name = strings.Trim(name, "_-")
	if name == "" {
		return "emoji"
	}
	return name
}

func getExtension(rawURL, contentType string) string {
	mediaType := strings.ToLower(strings.TrimSpace(strings.SplitN(contentType, ";", 2)[0]))
	extensions := map[string]string{
		"image/png": ".png", "image/gif": ".gif", "image/jpeg": ".jpg",
		"image/webp": ".webp", "image/svg+xml": ".svg",
	}
	if extension, ok := extensions[mediaType]; ok {
		return extension
	}
	if parsed, err := url.Parse(rawURL); err == nil {
		extension := strings.ToLower(filepath.Ext(parsed.Path))
		switch extension {
		case ".png", ".gif", ".jpg", ".jpeg", ".webp", ".svg":
			return extension
		}
	}
	return ".png"
}
