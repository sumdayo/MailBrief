# MailBrief

Gmail に届いた新着メールを検知し、LINE に即時通知するサーバーレスアプリケーションです。
未読メールを定期的にチェックし、新しいメールが届くと LINE に通知します。
一度通知したメールは重複して送らないように管理されています。

## アーキテクチャ

Google Cloud (GCP) を活用したサーバーレス構成です。

- **言語**: Go (1.22)
- **基盤**: Cloud Functions
- **トリガー**: Cloud Scheduler (定期実行)
- **データベース**: Firestore (通知済みメールの管理)
- **外部 API**:
  - Gmail API (メールの取得)
  - LINE Messaging API (通知送信)

## 目的

普段 LINE ばかり見ていて、Gmail の確認が遅れてしまう課題を解決するために作成しました。
スマホに標準で入っているメール通知よりも、使い慣れた LINE に流すことで、即座に内容を確認できるようにしました。

---

## 使い方 (ローカル開発)

1. リポジトリをクローン
2. 必要な環境変数を `.env` に設定
   ```
   GCP_PROJECT_ID=your-project-id
   LINE_CHANNEL_ACCESS_TOKEN=your-token
   LINE_USER_ID=your-user-id
   GMAIL_OAUTH_TOKEN_JSON={"type":"authorized_user","client_id":"...","client_secret":"...","refresh_token":"..."}
   ```
3. Cloud Functions にデプロイして実行

## 個人Gmail用の認証トークン生成

1. Google Cloud Console で OAuth クライアント（デスクトップ）を作成し、`credentials.json` を取得
2. トークン生成
   ```bash
   go run ./cmd/gmail-oauth
   ```
3. 生成された `token.json` の内容を `GMAIL_OAUTH_TOKEN_JSON` にセット
   （もしくは `GMAIL_OAUTH_TOKEN_FILE=token.json` を使う）

## メモ

- LINE Messaging APIでは、送信回数に対して月に200回までの上限が設けられている。なので他の通知方法を検討中。
- MailBriefという名前は、もともとメールの内容を要約した文章を送信するということで命名した。現状実現できていないが、今後AIによる要約機能も実装する予定。
- Webhookを用いて、ユーザーからのアクションを受け取る機能を実装したい。具体的に、通知メッセージの下に「既読にする」「返信案を作成」といったボタンを設置する。ボタンが押されたことを Webhook で検知し、Cloud Functions側で処理（Gmailの既読化APIを叩くなど）を行う。
