package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

func main() {
	os.Exit(run())
}

func run() int {
	options, err := parseOptions(os.Args[1:])
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		fmt.Fprintln(os.Stderr, err)
		return 2
	}
	if err := loadDotEnv(".env"); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}

	token := os.Getenv("SLACK_TOKEN")
	if token == "" {
		token = options.token
	}
	if token == "" {
		fmt.Fprintln(os.Stderr, "SLACK_TOKEN環境変数または-tokenが必要です")
		return 2
	}

	outputDir, err := filepath.Abs(options.output)
	if err != nil {
		fmt.Fprintf(os.Stderr, "出力先を解決できません: %v\n", err)
		return 1
	}

	client := newHTTPClient(30 * time.Second)
	ctx := context.Background()
	fmt.Println("Slackからカスタム絵文字一覧を取得しています…")
	emoji, err := listEmoji(ctx, client, token)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	fmt.Printf("%d件を取得しました。\n", len(emoji))

	manifest, err := exportEmoji(ctx, client, emoji, exportOptions{
		outputDir:   outputDir,
		concurrency: options.concurrency,
		overwrite:   options.overwrite,
		onProgress: func(completed, total int, item exportedEmoji) {
			detail := ""
			if item.Status == statusFailed {
				detail = ": " + item.Error
			}
			fmt.Printf("[%d/%d] %s: %s%s\n", completed, total, item.Name, item.Status, detail)
		},
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}

	fmt.Printf("\n完了: %s\n", outputDir)
	fmt.Printf(
		"downloaded=%d, skipped=%d, aliases=%d, failed=%d\n",
		manifest.Downloaded,
		manifest.Skipped,
		manifest.Aliases,
		manifest.Failed,
	)
	if manifest.Failed > 0 {
		return 1
	}
	return 0
}

type cliOptions struct {
	token       string
	output      string
	concurrency int
	overwrite   bool
}

func parseOptions(args []string) (cliOptions, error) {
	var options cliOptions
	flags := flag.NewFlagSet("slack-emoji-exporter", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	flags.StringVar(&options.token, "token", "", "Slack Bot/User OAuth Token（SLACK_TOKENを優先）")
	flags.StringVar(&options.output, "output", "./exported-emoji", "出力先")
	flags.IntVar(&options.concurrency, "concurrency", 5, "同時ダウンロード数（1〜50）")
	flags.BoolVar(&options.overwrite, "overwrite", false, "同名ファイルを上書きする")
	flags.Usage = func() {
		fmt.Fprintf(flags.Output(), "Slackワークスペースのカスタム絵文字を画像とJSONで保存します\n\n")
		fmt.Fprintf(flags.Output(), "Usage: slack-emoji-exporter [options]\n\nOptions:\n")
		flags.PrintDefaults()
	}

	if err := flags.Parse(args); err != nil {
		return cliOptions{}, err
	}
	if flags.NArg() != 0 {
		return cliOptions{}, fmt.Errorf("不明な引数です: %s", flags.Arg(0))
	}
	if options.concurrency < 1 || options.concurrency > 50 {
		return cliOptions{}, fmt.Errorf("-concurrencyには1〜50の整数を指定してください")
	}
	return options, nil
}
