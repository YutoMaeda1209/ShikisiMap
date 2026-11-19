# 改修概要

## 目的

下記の機能を追加する。

- スポットに訪問済みフラグを付けられるようにする
- Googleマップでスポットを検索するリンクを表示する

## 構成

- フロントエンド
  - マップを OpenStreetMap (Leaflet) で表示
  - 現在位置の表示
- バックエンド
  - ユーザー認証
  - スポット情報の管理 (座標・閉業情報はメンテナが手動で更新する)
  - 訪問済みスポット情報の管理

---

## 詳細仕様

### 1. 全体方針

| 項目 | 決定事項 |
| --- | --- |
| 地図 | 従来通り Leaflet + OpenStreetMap を使う。Google Maps Platform の API (Maps JavaScript API、Places API) は利用料金の問題から使わない |
| スポット情報の取得 | バックエンドは外部 API からスポット情報を取得せず、DB の内容だけを返す。座標と閉業情報はメンテナが手動で登録・更新する |
| スポットデータ | `src/frontend/src/data.json` から PostgreSQL に移行し、フロントは API から取得する |
| 認証 | Google OAuth2 ログインを独自実装する。将来 Firebase Auth へインポート/移行できる設計にする |
| セッション | DB で管理するセッション。セッション ID を HttpOnly Cookie で保持する |
| 未ログイン | 閲覧は従来通り誰でも可能。訪問記録は localStorage に保存でき、ログイン時にサーバーへ統合する |
| デプロイ | shikisimap VM の nginx で `/api/*` を同じ VM 上の Go バックエンドへ転送する同一オリジン構成。インフラは `home-server` リポジトリ (OpenTofu / Ansible) で管理する |

### 2. データモデル

「1 動画内の 1 箇所 = 1 スポット」の現行構造を、場所 (`spots`) と動画内の登場 (`appearances`) に分離する。訪問記録は場所に紐付く。

#### spots

| カラム | 型 | 備考 |
| --- | --- | --- |
| id | uuid PK | 既存 `data.json` の id を引き継ぐ |
| name | text | |
| address | text | |
| location | geography(Point, 4326) | 座標。移行時は `data.json` の値、登録時は住所から求めた値かメンテナの指定値。メンテナが手動で修正する |
| is_closed | boolean | 閉業フラグ。メンテナが手動で更新する |
| created_at / updated_at | timestamptz | |

#### appearances

| カラム | 型 | 備考 |
| --- | --- | --- |
| id | uuid PK | |
| spot_id | uuid FK → spots | |
| youtube_id | text | |
| timestamp | integer | 動画内の秒数 |
| UNIQUE (spot_id, youtube_id, timestamp) | | |

#### users

| カラム | 型 | 備考 |
| --- | --- | --- |
| id | uuid PK | Firebase 移行時に `uid` としてそのまま使う |
| google_sub | text UNIQUE | Google の `sub`。Firebase インポート時の `providerUserInfo.rawId` |
| email | text | |
| display_name | text | 公開用表示名。ユーザーが編集可能。初期値は Google のアカウント名 |
| profile_public | boolean | 既定 `false` |
| created_at / updated_at | timestamptz | |

#### visits

| カラム | 型 | 備考 |
| --- | --- | --- |
| id | uuid PK | |
| user_id | uuid FK → users (ON DELETE CASCADE) | |
| spot_id | uuid FK → spots | |
| visited_at | timestamptz | ユーザーが日付を指定可能。既定は登録時刻 |
| memo | text NULL | 本人のみ閲覧可 |
| created_at / updated_at | timestamptz | |
| UNIQUE (user_id, spot_id, visited_at) | | 重複登録の防止 |

同一スポットに複数回の訪問を記録できる。「訪問済み」は visits が 1 件以上存在することを指す。

#### sessions

| カラム | 型 | 備考 |
| --- | --- | --- |
| id | uuid PK | |
| user_id | uuid FK → users (ON DELETE CASCADE) | |
| token_hash | text UNIQUE | Cookie に入れるセッション ID のハッシュ。平文は保存しない |
| created_at | timestamptz | |
| last_used_at | timestamptz | |
| expires_at | timestamptz | |

期限切れのセッションは定期ジョブで削除する。

#### 既存データの移行

- 初回投入用コマンドで `data.json` を取り込む
- 各エントリを 1 つの spot + 1 つの appearance として登録する。統合はしない (`data.json` に同じ店舗の重複エントリは無い)
- 座標と閉業情報は `data.json` の値をそのまま使う
- 登録済みの spot は変更しない。何度実行しても結果は同じで、投入後の手動修正も上書きしない
- 移行完了後、`data.json` と `utils/addUuidToGeoJson.ts` は削除する

### 3. 認証

#### フロー

1. フロントが `GET /api/auth/google/login` に遷移
2. バックエンドが `state` (+ PKCE) を発行し Google の認可画面へリダイレクト
3. `GET /api/auth/google/callback` でコードを交換し、ID トークンを検証して `sub` でユーザーを検索/作成
4. セッションを作成してセッション ID を Cookie で発行し、フロントへリダイレクト
5. 以降のリクエストは Cookie で認証する。フロントはトークンを扱わない

#### セッション

- セッション ID: 暗号論的乱数 (32 バイト以上)。ログインのたびに新しく発行する
- Cookie: `HttpOnly; Secure; SameSite=Lax; Path=/api`
- 有効期限: 最終利用から 30 日のスライディング方式
- ログアウト: `DELETE /api/session` で現在のセッションを削除し Cookie を消す (`204 No Content`)
- アカウント削除時は該当ユーザーの全セッションを削除する

#### CSRF 対策

- `SameSite=Lax` に加え、更新系リクエスト (POST / PATCH / DELETE) は `Origin` ヘッダーが自サイトのものか検証する
- 更新系 API は `Content-Type: application/json` のみ受け付ける

#### Firebase Auth 移行への配慮

- ユーザー ID はアプリ側の `users.id` を正とし、Google の `sub` は別カラムで保持する
- 認証ミドルウェアは「リクエストから `users.id` を得る」インターフェースに閉じ込める
- 移行時は Firebase の ID トークンを受け取り、Firebase Admin SDK の `createSessionCookie` で発行したセッション Cookie を検証する方式に差し替える。フロントの API 呼び出しは Cookie 認証のまま変えずに済む

### 4. API

`/api/admin/*` 以外のパスは `認証` 列が「要」のものだけログインセッションを必須とする。

| メソッド | パス | 認証 | 内容 |
| --- | --- | --- | --- |
| GET | `/api/spots` | 不要 | 全スポット + appearances + 訪問者数 (集計値) |
| DELETE | `/api/session` | 要 | ログアウト (現在のセッションを削除) |
| GET | `/api/me` | 要 | 自分のユーザー情報 |
| PATCH | `/api/me` | 要 | 表示名、プロフィール公開設定の更新 |
| DELETE | `/api/me` | 要 | アカウント削除 (users を削除し visits / sessions もカスケード削除) |
| GET | `/api/me/visits` | 要 | 自分の訪問記録一覧。`Accept` ヘッダーで JSON / CSV を切り替える (エクスポートを兼ねる) |
| POST | `/api/me/visits` | 要 | 訪問記録の追加 (1 件以上をまとめて追加できる) |
| PATCH | `/api/me/visits/:id` | 要 | 訪問日時・メモの更新 |
| DELETE | `/api/me/visits/:id` | 要 | 訪問記録の削除 |
| GET | `/api/users/:id` | 不要 | 公開プロフィール (表示名 + 訪問済みスポット一覧)。非公開なら 404 |
| POST | `/api/admin/spots` | 管理 | スポット登録 (Issue フローから呼ぶ) |
| PATCH | `/api/admin/spots/:id` | 管理 | スポット編集 (閉業情報・座標・名前・住所の手動更新) |

#### 訪問記録の追加 (`POST /api/me/visits`)

- リクエストボディは訪問記録の配列とする。画面からの 1 件追加も要素 1 つの配列で送る
- localStorage からの取り込みも同じエンドポイントで全件をまとめて送る
- 同じユーザーで spot_id + visited_at が既存の記録と一致するものは重複として作成しない (UNIQUE 制約 + `ON CONFLICT DO NOTHING`)
- 全件を 1 トランザクションで処理する。存在しない spot_id があれば全件を作成せず `422` を返す
- レスポンスは `201 Created` で、作成された記録と重複でスキップされた件数を返す

#### 訪問記録の一覧 (`GET /api/me/visits`)

- `Accept: application/json` (既定) なら JSON、`Accept: text/csv` なら CSV を返す
- エクスポートはフロントが `Accept: text/csv` または `application/json` で取得し、ファイルとして保存させる

#### 管理 API

- 管理 API は GitHub Actions の OIDC トークンで認証する
  - ワークフローは `id-token: write` 権限で audience を指定して OIDC トークンを取得し、`Authorization: Bearer` で送る
  - バックエンドは `https://token.actions.githubusercontent.com` の JWKS で署名を検証し、`iss`、`aud`、`exp` を確認する
  - `repository` が `YutoMaeda1209/ShikisiMap`、`workflow_ref` がスポット登録ワークフロー (`spot_request.yml`) かスポット更新ワークフロー (`spot_update.yml`)、`ref` が `refs/heads/main` のものだけ許可する
- `POST /api/admin/spots`
  - 新規スポットは名前、住所、座標、閉業フラグ、動画 (YouTube ID と秒数) を受け取る
  - `spotId` を指定した場合、または名前と住所が一致するスポットがある場合は、新規スポットを作らず appearance だけを追加する
  - 新規スポットで座標が無い場合は `422` (`location_required`) を返す
- `PATCH /api/admin/spots/:id` は指定されたフィールドだけを更新する
- 公開プロフィールに訪問日時とメモは含めない

### 5. 地図とスポット情報

#### 地図表示 (フロント)

- 従来通り Leaflet (`react-leaflet`) で OpenStreetMap のタイルを表示する。帰属表示 (© OpenStreetMap contributors) を必ず出す
- 現在地の表示、選択中マーカーの強調、マップクリックでサイドバーを閉じる等の既存挙動は維持する
- マーカーは `L.divIcon` に HTML 要素を渡して描画する。
  Leaflet 既定のマーカー画像と `current_location.webp` は使わない
- 選択中のスポットはクラスタとは別のマーカーで描き、どのズームレベルでも表示する

#### マーカーデザイン

色は仮置きで、実装時に調整する。

**スポットマーカー**

- 形状: しずく型のピン (SVG、28 × 36px)。白の縁取り 2px と軽いドロップシャドウ
- ピン中央に状態を表すアイコンを白で描く (`bootstrap-icons` を利用)。色だけに頼らず、色覚特性によらず区別できるようにする

| 状態 | 本体色 | アイコン | その他 |
| --- | --- | --- | --- |
| 未訪問 | `#E5533D` (赤橙) | `bi-play-fill` (動画で紹介されたスポット) | |
| 訪問済み | `#2E9E5B` (緑) | `bi-check-lg` | |
| 閉業 | `#9AA0A6` (灰) | `bi-x-lg` | 不透明度 0.7、重なり順を最下位にする |
| 選択中 | 元の状態の色を維持 | 元の状態のアイコンを維持 | 1.3 倍に拡大し、重なり順を最上位にする。足元に波紋アニメーション |

- ホバー時はスポット名をツールチップで表示する (`title` 属性)
- `prefers-reduced-motion` が有効な場合はアニメーションを止める

**現在地マーカー**

- 青 (`#1A73E8`) の直径 16px の円 + 白縁 3px
- 周囲に位置情報の精度 (`accuracy`) を半径とする半透明の青い円を表示する
- 選択中マーカーと区別するため、波紋アニメーションは付けない

**クラスタ**

- `leaflet.markercluster` の `iconCreateFunction` で描画する
- 白地の円に件数を表示し、件数に応じて 3 段階 (〜9 / 10〜49 / 50〜) で大きさを変える
- 円の外周リングを、含まれるスポットの訪問済み割合に応じて緑で塗る (`conic-gradient`)。閉業スポットは割合の計算から除外する
- ズームレベル 15 以上ではクラスタリングしない

#### Google マップへのリンク (フロント)

- 営業時間などは Google マップの検索結果へのリンクで確認してもらう。リンク (Maps URLs) は API キー不要・無料
  - `https://www.google.com/maps/search/?api=1&query=<名前 住所>`
  - `query` は `encodeURIComponent` でエンコードする
- スポット選択時の表示は現在と同様に埋め込み動画とする

#### 座標・閉業情報の管理

- バックエンドは外部 API からスポット情報を取得しない。座標と閉業情報は DB の値を正とする
- 座標
  - 既存スポットは `data.json` の座標を引き継ぐ
  - 新規スポットはスポット登録フローで、住所から国土地理院の住所検索 API で求めるか、メンテナが指定する
- 閉業情報
  - 既存スポットは `data.json` の `isClosed` を引き継ぐ
  - 閉業が分かったら、メンテナがスポット更新ワークフロー (§7) で `is_closed` を更新する
- 座標の誤りも同じワークフローで修正する

### 6. 訪問記録 UI

#### スポット一覧の動画表示

- 1 スポットに複数の動画 (appearances) がある場合、一覧の項目内で動画を切り替えられる UI を付ける
- 既定では最新の動画を表示する

#### 訪問記録

- スポット詳細から「訪問を記録」でき、訪問日時 (既定は現在) とメモを入力できる
- 訪問履歴の一覧表示・編集・削除ができる
- サイドバーの一覧に「すべて / 訪問済み / 未訪問」のフィルタを追加する
- 「N / M 件 訪問済み」の達成率を表示する。閉業スポットは分母から除外する (閉業スポットへの訪問記録は分子にも数えない)
- 各スポットに訪問者数 (集計値) を表示する
- ユーザーメニュー: ログイン/ログアウト、表示名編集、プロフィール公開設定、エクスポート、アカウント削除
- 公開プロフィールページ (`/users/:id`) を追加する。`:id` は `users.id` (UUID) を使う

#### 未ログイン時の localStorage 保存

- 未ログインでも訪問記録 (spot_id、visited_at、memo) を localStorage に保存できる
- ログイン時にローカル記録があれば「この端末の記録 N 件を取り込みますか」と確認する
- 承認時は `POST /api/me/visits` で全件を送る。重複の除外はサーバー側で行う。成功後にローカル記録を削除する
- 拒否時はローカル記録を残す (破棄するボタンも用意する)

### 7. スポット登録フロー (GitHub Issue)

現行の `.github/scripts/process-spot-request.ts` (data.json に追記して PR を作成) を以下に置き換える。

1. 投稿者が `spot_request` テンプレートで Issue を作成する
2. 書き込み権限を持つメンテナ (`author_association` が OWNER / MEMBER / COLLABORATOR) が `/approve` とコメントする
   - `/approve location=<緯度>,<経度>` で座標を指定できる
   - `/approve spot_id=<id>` で登録済みのスポットに動画だけを追加できる
3. Actions が座標を求める (座標の指定が無い場合のみ)。国土地理院の住所検索 API (無料) で住所から求める
4. Actions が `POST /api/admin/spots` を呼ぶ
   - 登録できた場合: 結果 (座標の地図リンクを含む) を Issue にコメントしてクローズする
   - 座標を求められなかった新規スポットの場合: その旨を Issue にコメントして保留する。メンテナが座標を調べて `/approve location=<緯度>,<経度>` で再実行する
5. 名前と住所が登録済みのスポットと一致した場合は、新規 spot を作らず appearance だけを追加する

#### スポット更新ワークフロー

- `.github/workflows/spot_update.yml` (`workflow_dispatch`)。メンテナが Actions 画面から `main` で実行する
- 入力: スポット ID、閉業状態 (変更しない / 閉業 / 営業中)、座標、スポット名、住所。空欄の項目は変更しない
- `PATCH /api/admin/spots/:id` を OIDC トークンで呼ぶ
