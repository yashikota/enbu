# 💃 enbu

GitHubだけで完結する `.env` 管理ツール  

## なぜ

開発にはAPIキーやDBのパスワードといった機密情報が欠かせないが以下のような問題点がある  

- Slack/Discord/メールはE2EE非対応
    - 「1」「I」「l」のような紛らわしい文字や斜体表示による見間違いがエラーの温床にも
    - 変更のたびに全員へ連絡が必要で負荷が高い
    - 暗号化するにしても
        - 暗号化ファイルを送っても、パスワードや復号鍵の受け渡し経路が安全でない
- 専用品を使えば解決では？
    - 外部サービス利用時のコスト・運用負荷が課題
        - AWS/Google Cloud/1Password等の導入には契約やアカウント管理が必要
        - 運用面・金銭面の両方で組織の大きな負担に
- じゃあGitに含めたらいいじゃん！
    - Git履歴に機密情報の暗号文が永続的に残存
    - 将来的なアルゴリズム脆弱化により、後から解読される危険性

## 特徴

- **GitHubだけで完結** ほかのプラットフォームに依存することなく完結
- **E2E暗号化** 復号できるのは各メンバーのローカル秘密鍵のみ  
- **使いやすいCLI** セットアップを済ませれば `enbu add` と `enbu pull` だけすればいい  
<!-- 実装中-->
<!--- **機密情報の流出検知・防止** .env等の機密ファイルのcommitやべた書きを防止-->
<!--- **改ざん検知** Sigstoreによる署名と検証により改ざんを検知  -->
<!--- **ポリシー制御** OPA/Regoによるポリシー制御  -->

## インストール

```bash
go install github.com/yashikota/enbu@latest
```

または [Releases](https://github.com/yashikota/enbu/releases) からバイナリをダウンロード  

## クイックスタート

### 1. 認証

```bash
enbu auth login
```

GitHubにログインします  

### 2. リポジトリの初期化

```bash
cd your-repo
enbu init
```

各ユーザーごとにそのリポジトリで1度初期化をします  
以下が自動で行われます  

- X25519鍵ペアの生成
- 秘密鍵をOSキーチェーンに保存
- 公開鍵をGHCRに登録
- `enbu.toml` の作成
- `.gitignore` の更新

### 3. シークレットの追加

```bash
enbu add DATABASE_URL "postgres://..."
enbu add API_KEY "sk-..."
```

### 4. シークレットの取得

```bash
enbu pull # .env ファイルに書き出し
```

### 5. メンバーの追加

新しいメンバーがリポジトリ内で `enbu init` を実行すると、joinモードで公開鍵が登録されます。  
既存メンバーがローカルで `enbu sync` を実行すると、そのメンバーも復号可能になります。  

## 鍵の保管

秘密鍵は OS のセキュアストレージに保管されます  

| OS | バックエンド |
|----|-------------|
| macOS | Keychain |
| Linux | Secret Service (GNOME Keyring / KWallet) |
| Windows | Credential Manager |

キーチェーンが利用できない環境（コンテナ、ヘッドレスサーバー等）では、環境変数でフォールバックを指定できます  

```bash
export ENBU_BACKEND=text  # 平文ファイル (0600) で保存
```

## 仕組み

```
GHCR (ghcr.io/{owner}/{repo}-enbu)
├── recipient-{user}-{fingerprint}  ← 全メンバーの公開鍵
└── secrets-default                 ← 暗号化されたシークレット
```

1. `enbu add`  - シークレットを全受信者の公開鍵で暗号化し、OCI Imageアーティファクトとしてプッシュ  
2. `enbu pull` - 暗号文をプルし、自分の秘密鍵で復号して `.env` に書き出し  
3. `enbu sync` - メンバー追加・削除時に最新の受信者リストで再暗号化  

### 認証・初期化フロー

```mermaid
sequenceDiagram
    participant User as ユーザー
    participant CLI as enbu CLI
    participant GitHub as GitHub OAuth
    participant GHCR as GHCR

    User->>CLI: enbu auth login
    CLI->>GitHub: Device Code 要求
    GitHub-->>CLI: User Code + Verification URI
    CLI-->>User: コードを表示・ブラウザを開く
    User->>GitHub: ブラウザで認証・承認
    CLI->>GitHub: トークンをポーリング
    GitHub-->>CLI: Access Token
    CLI-->>User: ✓ Authenticated

    User->>CLI: enbu init
    CLI->>CLI: age X25519 鍵ペア生成
    CLI->>CLI: 秘密鍵を OS キーチェーンに保存
    CLI->>GHCR: recipient-{user}-{fingerprint} として公開鍵を登録
    GHCR-->>CLI: 完了
    CLI-->>User: ✓ Initialized
```

### シークレット追加フロー

```mermaid
sequenceDiagram
    participant User as ユーザー
    participant CLI as enbu CLI
    participant GHCR as GHCR

    User->>CLI: enbu add KEY VALUE
    CLI->>GHCR: 全 recipient の公開鍵を取得
    GHCR-->>CLI: 公開鍵リスト
    CLI->>CLI: age で全公開鍵向けに暗号化
    CLI->>GHCR: secrets-default にプッシュ
    GHCR-->>CLI: 完了
    CLI-->>User: ✓ Secret added
```

### メンバー追加・同期フロー

```mermaid
sequenceDiagram
    participant New as 新メンバー
    participant Member as 既存メンバー
    participant CLI as enbu CLI
    participant GHCR as GHCR

    New->>CLI: enbu init (join mode)
    CLI->>CLI: age 鍵ペア生成
    CLI->>GHCR: recipient-{user} として公開鍵を登録
    CLI-->>New: ✓ 鍵を登録しました

    Member->>CLI: enbu sync
    CLI->>GHCR: 全 recipient の公開鍵を取得
    GHCR-->>CLI: 公開鍵リスト
    CLI->>GHCR: secrets-default をプル
    GHCR-->>CLI: 暗号文
    CLI->>CLI: メンバーの秘密鍵で復号 → 全公開鍵で再暗号化
    CLI->>GHCR: secrets-default を更新

    New->>CLI: enbu pull
    CLI->>GHCR: secrets-default をプル
    GHCR-->>CLI: 暗号文
    CLI->>CLI: 自分の秘密鍵で復号
    CLI-->>New: .env に書き出し
```
