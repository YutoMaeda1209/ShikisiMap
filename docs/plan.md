# 実装計画

[`docs/spec.md`](./spec.md) の改修を実装するための計画。

- 各フェーズは Issue を作成し、`main` から `feat/{IssueNum}-{featureName}` ブランチを切って `main` 向けの PR にする ([CONTRIBUTING](../.github/CONTRIBUTING.md))
- インフラの変更は [`home-server`](https://github.com/YutoMaeda1209/home-server) リポジトリで行う。`main` への push で本番に適用されるため、事前に `--check --diff` で確認する

## 依存関係

```
P0 準備 (手作業) ─ P1 インフラ (home-server)
P1 バックエンド基盤 ─┬ P2 スポット API・データ投入 ─┬ P3 フロントの Google Maps 移行
                     │                              ├ P7 定期ジョブ
                     │                              └ P8 スポット登録フロー
                     └ P4 認証 ─ P5 訪問記録 ─ P6 公開プロフィール
P9 デプロイ・切り替え (全フェーズ完了後)
```

P3 と P4 は P2 の完了後に並行して進められる。インフラ (P1 インフラ) はアプリの実装と並行して進め、P9 までに完了させる。

## P0 準備 (手作業)

- Google Cloud
  - OAuth クライアント (ウェブアプリケーション) を作成し、リダイレクト URI に以下を登録する
    - `http://localhost:5173/api/auth/google/callback`
    - `https://map.mendako.dev/api/auth/google/callback`
  - API キーを 2 つ作成する
    - フロント用: HTTP リファラー制限 (`https://map.mendako.dev/*`、`http://localhost:5173/*`)、Maps JavaScript API のみ
    - バックエンド用: IP 制限 (自宅回線の グローバル IP)、Places API (New) のみ
  - Map ID を発行し、クラウドスタイルで店舗 POI アイコンを非表示にする
  - 予算アラートと割り当て上限を設定する (spec §5「費用管理」)
- GitHub (ShikisiMap リポジトリ)
  - Variables: `VITE_GOOGLE_MAPS_API_KEY`、`VITE_GOOGLE_MAP_ID`
  - スポット登録ワークフロー用の OIDC audience を決める (例: `https://map.mendako.dev/api/admin`)

## P1 インフラ (`home-server`)

`home-server` の `shikisimap-backend` ブランチに変更を用意済み (未 push)。`main` への push で本番に適用されるため、**バックエンドの実装 (P1〜P8) が終わってから適用する**。

HAProxy はホスト名で shikisimap VM の 80 番へ転送しているだけなので変更不要。変更は shikisimap VM 内に閉じる。

- VM: `opentofu/proxmox/vms.tofu` で shikisimap のメモリを 512MB → 1024MB (適用時に VM の再起動を伴う可能性がある)
- ロール `postgresql` (新規)
  - PGDG の apt リポジトリから PostgreSQL 18 と PostGIS 3 を入れる (開発環境の `postgis/postgis:18-3.7` に合わせる)
  - `host_vars/shikisimap.yml` の `postgresql_databases` に従い、ロール兼 DB `shikisimap` を作成し、`postgis` 拡張をスーパーユーザーで作成する
  - Ansible コレクション `community.postgresql` を追加
- ロール `shikisimap_backend` (新規)
  - システムユーザー `shikisimap`、バイナリ置き場 `/srv/backend` (`deploy` 所有)
  - `/etc/shikisimap/backend.env` (`root:shikisimap 0640`) に以下を出力する
    - `LISTEN_ADDR=127.0.0.1:8080`、`DATABASE_URL`、`PUBLIC_ORIGIN=https://map.mendako.dev`、`ADMIN_OIDC_AUDIENCE=https://map.mendako.dev/api/admin`
    - `GOOGLE_OAUTH_CLIENT_ID`、`GOOGLE_OAUTH_CLIENT_SECRET`、`GOOGLE_PLACES_API_KEY`
  - `shikisimap-backend.service`: バイナリが置かれるまでは起動しない (`ConditionPathExists`)
  - `shikisimap-backend-restart.path`: バイナリの更新を検知して再起動する (deploy ユーザーに sudo を与えないため)
- ロール `nginx_static` に変数を追加 (Blog は従来の設定のまま)
  - `nginx_static_spa_fallback`: `try_files ... /index.html` で `/users/:id` に対応
  - `nginx_static_api_upstream`: `/api/` を `127.0.0.1:8080` へ転送
- バックアップ: shikisimap を `backup` グループに追加し、`pg_dump --format=custom` を restic で R2 へ保存する。リストアテストではダンプの目次に主要テーブルがあるかを確認する
- 秘密情報は vault に置く (`vault.example.yml` に追記済み)。`vault_shikisimap_db_password` が空の間は PostgreSQL、バックエンド、バックアップのいずれも構成されないため、他の変更と一緒に `main` に入っても影響しない
  - `vault_shikisimap_db_password`、`vault_shikisimap_google_oauth_client_id`、`vault_shikisimap_google_oauth_client_secret`、`vault_shikisimap_google_places_api_key`

適用手順

1. vault に上記 4 つの値を設定する
2. `ansible-playbook playbooks/site.yml --check --diff` と `tofu plan` で差分を確認する
3. `main` にマージして適用し、ShikisiMap 側の P9 でバックエンドをデプロイする

## P1 バックエンド基盤

- `src/backend/main.go` を分割し、`internal/` 配下にパッケージを置く
  - `config` (環境変数。`LISTEN_ADDR`、`DATABASE_URL`、`PUBLIC_ORIGIN`、`ADMIN_OIDC_AUDIENCE`、`GOOGLE_*`。名前は `home-server` の `backend.env` と合わせる)、`db`、`httpx` (共通ミドルウェア、エラー形式)、`auth`、`spots`、`visits`、`users`、`admin`、`places`、`scheduler`
- マイグレーション `00002_create_tables.sql`
  - `spots`、`appearances`、`users`、`visits`、`sessions` とインデックス
  - `visits` に `UNIQUE (user_id, spot_id, visited_at)`
- 共通ミドルウェア
  - 更新系 (POST / PATCH / DELETE) の `Origin` 検証と `Content-Type: application/json` の強制
  - エラーレスポンス形式の統一 (`{"error": {"code": ..., "message": ...}}`)
- 開発環境
  - `vite.config.ts` に `/api` → `http://localhost:8080` のプロキシを追加し、開発時も同一オリジンにする
  - これにより CORS が不要になるため、`gin-contrib/cors` と `CORS_ALLOW_ORIGINS` を削除する
- CI (`build_test.yml`)
  - PostGIS のサービスコンテナを使った `go test ./...` を追加する
  - フロントに `npm run lint` を追加する

## P2 スポット API・既存データの投入

- `internal/places`: Places API (New) クライアント
  - Text Search: `places.id`、`places.displayName`、`places.formattedAddress`、`places.location`
  - Place Details: `location`、`businessStatus`
- `GET /api/spots`: スポット一覧 + appearances + 訪問者数
- 投入コマンド `shikisimap-backend import-spots <data.json>`
  - goose のマイグレーション内で外部 API を呼ばないよう、別のサブコマンドにする
  - `--dry-run` で place_id の解決結果を出力し、以下を一覧にする
    - 解決できなかったもの (`place_id = NULL` で登録し、既存座標を使う)
    - 同じ place_id に解決されて統合されるもの
  - 統合候補はメンテナが確認し、誤りがあれば上書き用の対応表 (`data.json` の id → place_id) を渡して再実行する
  - 何度実行しても同じ結果になるようにする (spot id は `data.json` の id を使う)

## P3 フロントの Google Maps 移行

- 依存の入れ替え
  - 追加: `@vis.gl/react-google-maps`、`@googlemaps/markerclusterer`
  - 削除: `leaflet`、`react-leaflet`、`@types/leaflet`、`uuid`
- データ取得
  - `MapDataProvider` を `GET /api/spots` から取得する形に変更する
  - `GeoProperties` を新しいモデル (spot + appearances) の型に置き換える
- 地図 (`Map.tsx`、`CurrentLocationMarker.tsx` の書き直し)
  - `AdvancedMarkerElement` による状態別マーカー、選択中の強調 (spec §5「マーカーデザイン」)
  - 現在地マーカーと精度円
  - クラスタのカスタムレンダラー (件数、訪問済み割合のリング)
  - マップクリックでサイドバーを閉じる、選択時のパンなど既存挙動の移植
- 一覧 (`SpotItem.tsx`)
  - 複数の appearances がある場合の動画切り替え UI (既定は最新の動画)
  - 住所リンクを place_id 付きの Maps URLs に変更する
- `deploy.yml` のビルドに `VITE_GOOGLE_MAPS_API_KEY` と `VITE_GOOGLE_MAP_ID` を渡す
- `Map.css` などの Leaflet 向けスタイルを削除する

## P4 認証

- `GET /api/auth/google/login`
  - `state` と PKCE の `code_verifier` を短命 (10 分) の HttpOnly Cookie に保存してリダイレクトする
- `GET /api/auth/google/callback`
  - `state` 検証、コード交換、ID トークン検証 (`github.com/coreos/go-oidc`)
  - `google_sub` でユーザーを検索・作成し、セッションを作成して Cookie を発行する
- セッション
  - セッション ID は 32 バイトの乱数、DB には SHA-256 ハッシュを保存する
  - 認証ミドルウェアで検証し、`last_used_at` と `expires_at` (最終利用から 30 日) を更新する。毎リクエストの書き込みを避けるため、更新は一定間隔 (例: 1 時間) ごとにする
  - `DELETE /api/session`
- `GET` / `PATCH` / `DELETE /api/me`
- フロント
  - `AuthProvider` (`GET /api/me` でログイン状態を取得)
  - ユーザーメニュー: ログイン / ログアウト、表示名編集、プロフィール公開設定、アカウント削除 (確認ダイアログ付き)

## P5 訪問記録

- バックエンド
  - `GET /api/me/visits` (`Accept` で JSON / CSV)
  - `POST /api/me/visits` (配列、1 トランザクション、`ON CONFLICT DO NOTHING`、存在しない spot_id は `422`)
  - `PATCH` / `DELETE /api/me/visits/:id`
  - `GET /api/spots` の訪問者数の集計
- フロント
  - 訪問記録ストアを抽象化し、未ログイン時は localStorage、ログイン時は API を使う
  - 訪問記録ダイアログ (訪問日時、メモ) と訪問履歴の表示・編集・削除
  - 一覧の「すべて / 訪問済み / 未訪問」フィルタ
  - 達成率表示 (閉業スポットを除外)
  - マーカーとクラスタへの訪問状態の反映
  - ログイン時の localStorage 取り込み確認
  - エクスポート (JSON / CSV のダウンロード)

## P6 公開プロフィール

- `GET /api/users/:id` (非公開なら 404)
- フロントに `/users/:id` ページを追加する
  - ページは 2 つだけなので、ルーターライブラリは入れず `UrlManager.tsx` を拡張して対応する
  - 表示名と訪問済みスポットの一覧・地図表示

## P7 定期ジョブ

- `internal/scheduler`: goroutine + ticker で 1 日 1 回実行する
  - `location_fetched_at` が 25 日以上前のスポットの座標を再取得する
  - `businessStatus` が `CLOSED_PERMANENTLY` なら `is_closed = true` にする
  - 1 回の処理件数の上限 (既定 20 件、環境変数で変更可)
  - 期限切れセッションの削除
- PostgreSQL の advisory lock で二重実行を防ぐ

## P8 スポット登録フロー

- 管理 API の認証ミドルウェア
  - GitHub Actions の OIDC トークンを `go-oidc` で検証し、`aud`、`repository`、`workflow_ref`、`ref` を確認する
- `POST /api/admin/spots`
  - place_id を解決して spot と appearance を登録する。既存の place_id と一致すれば appearance のみ追加する
  - 候補が 0 件または複数の場合は `409 Conflict` と候補一覧を返す
  - リクエストで place_id を指定された場合は解決を省略する
- `PATCH /api/admin/spots/:id`
- `.github/workflows/spot_request.yml`
  - トリガーを `issue_comment` (`created`) に変更する
  - コメントが `/approve` で始まり、`author_association` が OWNER / MEMBER / COLLABORATOR の場合のみ実行する
  - 権限を `id-token: write`、`issues: write` にする
- `.github/scripts/process-spot-request.ts`
  - Issue 本文を解析して管理 API を呼び、結果を Issue にコメントしてクローズする
  - `409` の場合は候補をコメントして保留する。`/approve place_id=<id>` で再実行できるようにする
  - data.json の更新と PR 作成の処理を削除する

## P9 デプロイ・切り替え

1. `deploy.yml` にバックエンドのビルドと rsync (`/srv/backend`) を追加する
2. バックエンドを先にデプロイし、`import-spots --dry-run` → 確認 → 本実行
3. 解決できなかったスポットを手動で解決する
4. フロントをデプロイする
5. `data.json`、`utils/addUuidToGeoJson.ts`、Leaflet 関連のアセットを削除する
6. README (技術スタック、クイックスタート、主なファイル) を更新する

## 補足: 既存スポット ID について

`data.json` の各スポットは最初に登場した動画のみを登録しているため、同じ店舗が複数エントリになっていることはなく、移行で消える ID は無い。旧 ID → 新 ID の対応表は作らない。移行直後の appearances は 1 スポットにつき 1 件になる。

座標が一致するエントリが 3 組あるが、いずれも同じ建物内の別店舗 (例: すみだ水族館とペンギンカフェ) である。誤って同じ place_id に解決されて統合されないよう、P2 の `--dry-run` で統合候補を確認する。
