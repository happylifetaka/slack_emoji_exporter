# Slack Emoji Exporter

Slackワークスペースのカスタム絵文字を、元の画像形式のままローカルへ保存するGo製CLIです。絵文字名・元URL・alias・ダウンロード結果は`result.json`にも保存します。

外部ライブラリは使用していません。ビルド後はGoやNode.jsが入っていない環境でも、生成した単一バイナリだけで実行できます。

## 必要なもの

- ビルド時: Go 1.22以上
- 実行時: Slack AppのBot User OAuth Token（`xoxb-...`）
- Slack Appに付与するBot Token Scope: `emoji:read`

## Slack Appの準備

1. [Slack APIのYour Apps](https://api.slack.com/apps)で「Create New App」を選びます。
2. 「From an app manifest」を選び、このリポジトリの`slack-app-manifest.yml`を貼り付けます。
3. 「OAuth & Permissions」から対象ワークスペースへAppをインストールします。
4. 表示された「Bot User OAuth Token」を控えます。

このCLIは読み取り専用の`emoji:read`だけを利用します。トークンはソースコードやGitに保存しないでください。

## ビルドと実行

最初に`.env.example`をコピーし、 `.env`に取得したSlack Tokenを設定します。

```bash
cp .env.example .env
```

```dotenv
SLACK_TOKEN=xoxb-your-token
```

その後、ビルドして実行します。起動時にカレントディレクトリの`.env`を自動で読み込みます。

```bash
go build -o bin/slack-emoji-exporter .
./bin/slack-emoji-exporter
```

出力先や並列数を変える場合:

```bash
./bin/slack-emoji-exporter \
  -output ./backup/emoji \
  -concurrency 8
```

既存の画像はデフォルトでスキップします。上書きする場合は`-overwrite`を追加します。シェルで設定した`SLACK_TOKEN`は`.env`の値より優先されます。`-token`でも渡せますが、シェル履歴やプロセス一覧に残り得るため`.env`を推奨します。

## 出力

```text
exported-emoji/
├── emoji/
│   ├── party_parrot.gif
│   └── wave.png
└── result.json
```

aliasは画像を複製せず、`result.json`に`"status": "alias"`と`"aliasFor"`を記録します。一部の画像取得に失敗した場合も結果を書き出し、CLIは終了コード1を返します。Slack公式は128 KB以下の画像を推奨していますが、既存絵文字との互換性に余裕を持たせ、ダウンロード上限は1件あたり1 MiBにしています。

## テスト

```bash
go test ./...
go vet ./...
```

## クロスコンパイル例

```bash
GOOS=linux GOARCH=amd64 go build -o bin/slack-emoji-exporter-linux-amd64 .
GOOS=darwin GOARCH=arm64 go build -o bin/slack-emoji-exporter-darwin-arm64 .
GOOS=windows GOARCH=amd64 go build -o bin/slack-emoji-exporter.exe .
```

## 免責事項

このプロジェクトは非公式ツールであり、Slack Technologies, LLCおよびSalesforce, Inc.とは関係ありません。Slackは各権利者の商標または登録商標です。

利用者自身の責任で使用し、Slackの利用規約、所属組織のポリシー、および適用される法令を遵守してください。作者は、本ツールの利用または利用不能によって生じたデータ損失、情報漏洩、アカウントへの影響、その他の損害について責任を負いません。

エクスポートされる絵文字には第三者が権利を有する画像が含まれる場合があります。複製、公開、再配布などを行う前に、必要な権利や許諾があることを確認してください。Slack Token、`result.json`、エクスポート画像にはワークスペース固有の情報が含まれ得るため、公開リポジトリへコミットしないでください。

## ライセンス

このソフトウェアは[MIT License](./LICENSE)で提供されます。
