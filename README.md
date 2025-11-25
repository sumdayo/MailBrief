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
   ```
3. 実行
   ```bash
   go run main.go
   ```

## メモ

- LINE Messaging APIでは、送信回数に対して月に200回までの上限が設けられている。なので他の通知方法を検討中。
- MailBriefという名前は、もともとメールの内容を要約した文章を送信するということで命名した。現状実現できていないが、今後AIによる要約機能も実装する予定。
