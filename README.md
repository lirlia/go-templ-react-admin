# go-templ-react-admin

`go templ` (MPA) を軸にしつつ、一覧操作など“ユーザーインタラクティブ”な部分だけ React を載せた **管理ツールのサンプル**です。

## できること（最小セット）

- **認証**: セッションログイン/ログアウト
- **Users 管理**（React）:
  - 検索 / ページング
  - ロール変更（adminのみ）
  - 有効/無効切り替え（adminのみ）
  - パスワードリセット（adminのみ）
- **Projects 管理**（React）:
  - 検索 / ページング
  - 作成（admin/editor）
- **監査ログ**: login / role変更 / active切替 / project作成 などを `audit_log` に記録

## 起動方法（開発）

フロント（Vite）とバックエンド（Go）を別プロセスで起動します。

### 1) フロントを起動

```bash
cd web
npm install
npm run dev
```

### 2) バックエンドを起動

別ターミナルで:

```bash
go run ./cmd/server -dev=true -seed=true
```

ブラウザで `http://127.0.0.1:8080/login` を開き、以下でログインできます。

- **email**: `admin@example.com`
- **password**: `admin`

## ビルド（本番想定）

```bash
cd web
npm run build
cd ..
go build ./cmd/server
./server -dev=false
```

`web` のビルド成果物は `static/assets/` に出力され、Go側が `/assets/` で配信します。

