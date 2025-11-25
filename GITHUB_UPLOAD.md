# GitHub へのアップロード手順

このドキュメントでは、MailBrief プロジェクトを GitHub にアップロードする手順を説明します。

## 前提条件

- GitHub アカウントを持っていること
- Git がインストールされていること（✅ 完了）
- ローカルリポジトリが初期化されていること（✅ 完了）

## 手順

### 1. GitHub で新しいリポジトリを作成

1. [GitHub](https://github.com) にログインします
2. 右上の「+」ボタンをクリックし、「New repository」を選択します
3. リポジトリ情報を入力します：
   - **Repository name**: `mailbrief` または任意の名前
   - **Description**: `Gmail の未読メールを LINE に通知するサーバーレスアプリケーション`
   - **Public/Private**: お好みで選択
   - **⚠️ 重要**: 「Add a README file」「Add .gitignore」「Choose a license」は**チェックしない**でください（既にローカルに存在するため）
4. 「Create repository」をクリックします

### 2. リモートリポジトリを追加

GitHub でリポジトリを作成すると、リモートリポジトリの URL が表示されます。
以下のコマンドを実行してリモートリポジトリを追加します：

```bash
# HTTPS を使用する場合
git remote add origin https://github.com/YOUR_USERNAME/mailbrief.git

# または SSH を使用する場合（SSH キーを設定済みの場合）
git remote add origin git@github.com:YOUR_USERNAME/mailbrief.git
```

> **注**: `YOUR_USERNAME` を実際の GitHub ユーザー名に置き換えてください。

### 3. プッシュ

```bash
git push -u origin main
```

初回プッシュ時に認証情報を求められる場合があります。

### 4. 確認

GitHub のリポジトリページにアクセスして、ファイルが正しくアップロードされていることを確認します。

## セキュリティチェックリスト

アップロード前に、以下の機密情報が除外されていることを確認してください：

- ✅ `.env` ファイル（環境変数）
- ✅ `client_secret.json`（Google Cloud 認証情報）
- ✅ その他の認証トークンや API キー

これらのファイルは `.gitignore` に記載されているため、自動的に除外されます。

## トラブルシューティング

### 認証エラーが発生する場合

GitHub の認証方法が変更されたため、パスワードの代わりに Personal Access Token (PAT) を使用する必要があります。

1. [GitHub Settings > Developer settings > Personal access tokens](https://github.com/settings/tokens) にアクセス
2. 「Generate new token (classic)」をクリック
3. 必要な権限（`repo` スコープ）を選択
4. トークンを生成し、コピーします
5. `git push` 時にパスワードの代わりにこのトークンを使用します

### SSH を使用する場合

SSH キーを設定していない場合は、[GitHub の SSH キー設定ガイド](https://docs.github.com/ja/authentication/connecting-to-github-with-ssh)を参照してください。
