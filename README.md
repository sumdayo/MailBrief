# MailBrief

MailBrief は、Gmail の未読メールを定期的にチェックし、LINE アプリに通知を送信するサーバーレスアプリケーションです。

> **注**: 現在のバージョンは、メール内容をそのまま LINE に送信します。将来的には Vertex AI (Gemini) を使用した要約機能を追加予定です。

## 機能 (Features)

- ✅ Gmail の未読メールを自動取得
- ✅ 特定ドメインからのメールを除外（スパム対策）
- ✅ LINE への即時通知
- ✅ Firestore による処理済みメールの管理（重複通知防止）
- 🚧 Vertex AI による要約機能（実装予定）

## アーキテクチャ (Architecture)

```mermaid
graph TD
    Scheduler[Cloud Scheduler] -->|定期的にトリガー| Function[Cloud Function\n(Go)]
    Function -->|1. 未読取得| Gmail[Gmail API]
    Gmail -->|メール内容| Function
    Function -->|2. 通知送信| LINE[LINE Messaging API]
    Function -->|3. 状態更新| Firestore[Firestore\n(mail_state)]
```

### コンポーネント

- **Cloud Scheduler**: メールチェック処理を定期的（例: 15 分ごと）にトリガーします。
- **Cloud Functions (Gen 2)**: Go 言語で記述されたコアロジックです。プロセス全体を制御します。
- **Gmail API**: 未読メールを取得します。
- **LINE Messaging API**: メール通知を LINE アカウントに送信します。
- **Firestore**: 重複処理を防ぐために、最後に処理したメールのタイムスタンプを保存します。

## 1. 前提条件 (Prerequisites)

利用するには以下の Google Cloud (GCP) と LINE の設定が必要です。

### Google Cloud Project

以下の API を有効化してください:

- Cloud Functions API
- Gmail API
- Firestore API
- Cloud Scheduler API
- Cloud Build API

コマンドで有効化する場合:

```bash
gcloud services enable \
  cloudfunctions.googleapis.com \
  run.googleapis.com \
  cloudbuild.googleapis.com \
  firestore.googleapis.com \
  gmail.googleapis.com \
  cloudscheduler.googleapis.com
```

**重要: Gmail API へのアクセス権**

- **Google Workspace (企業・組織向け)**:
  - サービスアカウントを作成し、ドメイン全体の委任 (Domain-Wide Delegation) を設定することを推奨します。これにより、ユーザーの介入なしにメールにアクセスできます。
- **個人の Gmail (@gmail.com)**:
  - 現在のコードは Application Default Credentials (ADC) を使用しています。個人の Gmail アカウントで Cloud Functions から直接アクセスする場合、セキュリティ上の制限により ADC だけでは動作しないことがあります。
  - その場合、OAuth 2.0 クライアント ID を作成し、リフレッシュトークンを使用して認証するロジックへの変更が必要になる場合があります。

### LINE Messaging API

1. [LINE Developers コンソール](https://developers.line.biz/)にログインします。
2. 新規プロバイダーを作成し、Messaging API チャネルを作成します。
3. **Channel Access Token (長期)** を発行します。
4. **Your User ID** を確認します（テスト送信に必要です）。

### Firestore

Firestore データベースを作成します（ネイティブモード）:

**コマンドラインで作成する場合:**

```bash
gcloud firestore databases create --location=asia-northeast1
```

**コンソールで作成する場合:**

1. [Firestore コンソール](https://console.cloud.google.com/firestore)にアクセスします。
2. データベースを作成します（ネイティブモード）。
3. リージョンを選択します（例: `asia-northeast1`）。

> **注**: `mail_state` コレクションは、アプリケーションが初回実行時に自動的に作成します。手動で作成する必要はありません。

## 2. ローカルでの開発 (Local Development)

1. `.env.example` を `.env` にコピーし、値を設定します。

   ```bash
   cp .env.example .env
   # .env を編集して実際の値を入力してください
   ```

2. ローカルサーバーを起動します。

   ```bash
   go run main.go
   # ポートを指定する場合: PORT=8080 go run main.go
   ```

3. 別のターミナルから関数をトリガーします。

   ```bash
   curl localhost:8080
   ```

## 3. Cloud Function のデプロイ (Deploy Cloud Function)

プレースホルダーを実際の値に置き換えて実行してください。

```bash
export GCP_PROJECT_ID="your-project-id"
export LINE_CHANNEL_ACCESS_TOKEN="your-channel-access-token"
export LINE_USER_ID="your-user-id"

gcloud functions deploy mailbrief \
  --gen2 \
  --runtime=go122 \
  --region=us-central1 \
  --source=. \
  --entry-point=ProcessEmails \
  --trigger-http \
  --set-env-vars=GCP_PROJECT_ID=$GCP_PROJECT_ID,LINE_CHANNEL_ACCESS_TOKEN=$LINE_CHANNEL_ACCESS_TOKEN,LINE_USER_ID=$LINE_USER_ID
```

## 4. Cloud Scheduler の設定 (Set up Cloud Scheduler)

関数を 15 分ごとにトリガーするジョブを作成します。

```bash
# 関数のURLを取得
FUNCTION_URL=$(gcloud functions describe mailbrief --gen2 --region=us-central1 --format="value(serviceConfig.uri)")

# スケジューラジョブを作成
gcloud scheduler jobs create http mailbrief-trigger \
  --schedule="*/15 * * * *" \
  --uri=$FUNCTION_URL \
  --http-method=GET \
  --location=us-central1 \
  --oidc-service-account-email=$(gcloud config get-value account)
```

_注意: Scheduler で使用するサービスアカウントには `Cloud Functions Invoker` 権限が必要です。_
