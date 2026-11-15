# 改修概要

## 目的

下記の機能を追加する。

- スポットに訪問済みフラグを付けられるようにする
- Googleマップのプレイス情報へのリンクを表示する

## 構成

- フロントエンド
  - マップを GoogleMap で表示
  - 現在位置の表示
- バックエンド
  - ユーザー認証
  - スポット情報の管理
  - 訪問済みスポット情報の管理
  - GoogleMapsApi から各スポットの座標の取得

---

## 詳細仕様

### 1. 全体方針

| 項目 | 決定事項 |
| --- | --- |
| 地図 | Leaflet + OSM から Google Maps (Maps JavaScript API) に完全移行し、Leaflet 関連の依存を削除する |
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
| id | uuid PK | 既存 `data.json` の id を引き継ぐ (同一 place_id で統合されたものは代表の 1 つ) |
| place_id | text UNIQUE NULL | Google の place_id。永続保存可 |
| name | text | |
| address | text | |
| location | geography(Point, 4326) | Google から取得した座標。30 日以内に再取得する |
| location_fetched_at | timestamptz | 座標の取得日時 |
| is_closed | boolean | 定期ジョブで businessStatus から自動更新 |
| status_checked_at | timestamptz | 営業状態の確認日時 |
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

- goose マイグレーション (または初回投入用コマンド) で `data.json` を取り込む
- 同一店舗の判定は place_id 解決後に行い、同じ place_id のものは 1 つの spot + 複数 appearance に統合する
- 統合対象は一覧で出力し、メンテナが確認してから反映する (同じ建物内の別店舗を誤って同じ place_id に解決する場合があるため)
- place_id が解決できなかったものは `place_id = NULL` で登録し、既存座標を暫定で使う。一覧化してメンテナが手動で解決する
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
| PATCH | `/api/admin/spots/:id` | 管理 | スポット編集 |

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
  - `repository` が `YutoMaeda1209/ShikisiMap`、`workflow_ref` がスポット登録ワークフロー、`ref` が `refs/heads/main` のものだけ許可する
- 公開プロフィールに訪問日時とメモは含めない

### 5. Google Maps Platform

#### 地図表示 (フロント)

- `@vis.gl/react-google-maps` を使う (仮)
- 現在地の表示、選択中マーカーの強調、マップクリックでサイドバーを閉じる等の既存挙動は維持する
- Map ID を Cloud Console で発行し、環境変数で渡す。開発中は `DEMO_MAP_ID` を使う
- Map ID のクラウドスタイルで、Google 標準の店舗 POI アイコンを非表示にする (スポットとの混同を避けるため)
- マーカーは `AdvancedMarkerElement` に HTML 要素を渡して描画する。
  既存の Leaflet マーカー画像と `current_location.webp` は流用せず削除する

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

- `@googlemaps/markerclusterer` のカスタムレンダラーで描画する
- 白地の円に件数を表示し、件数に応じて 3 段階 (〜9 / 10〜49 / 50〜) で大きさを変える
- 円の外周リングを、含まれるスポットの訪問済み割合に応じて緑で塗る (`conic-gradient`)。閉業スポットは割合の計算から除外する
- ズームレベル 15 以上ではクラスタリングしない

#### プレイス情報 (フロント)

- Places API でプレイス情報を取得・表示はしない。営業時間などは Google マップへのリンクで代用する (課金対策)
- 現在の住所リンクを place_id 付きの Maps URLs に置き換える
  - place_id あり: `https://www.google.com/maps/search/?api=1&query=<名前 住所>&query_place_id=<place_id>`
  - place_id が `NULL`: 現在と同様に `https://www.google.com/maps/search/?api=1&query=<名前 住所>`
  - `query` は `encodeURIComponent` でエンコードする
- 営業状態は DB の `is_closed` を使う
- スポット選択時の表示は現在と同様に埋め込み動画とする

#### 座標取得・営業状態更新 (バックエンド)

- スポット登録時に名前 + 住所で Places API (New) Text Search を行い place_id を確定する
- バックエンド内のスケジューラ (goroutine + ticker) で定期実行する
  - `location_fetched_at` が 30 日に近いものの座標を再取得する
  - `businessStatus` が `CLOSED_PERMANENTLY` なら `is_closed = true` にする
- 1 日 1 回程度、レート制限を考慮して少しずつ処理する
- 1 回の実行で処理する件数の上限を設ける (既定 20 件、環境変数で変更可)。後述の割り当て上限に達する前に止めるため

#### API キー

- フロント用: HTTP リファラー制限、Maps JavaScript API のみ許可
- バックエンド用: IP 制限、Places API (New) のみ許可
- キーは環境変数で渡し、リポジトリに含めない

#### 費用管理 (Cloud Console の運用設定)

想定費用は月 $0 のため、課金が発生した時点で異常とみなす。

**予算アラート** (Cloud Billing → 予算とアラート)

- 予算額: 月 $5
- 通知: 実績が 50% / 90% / 100%、および予測が 100% に達したとき
- 通知先: 請求先アカウント管理者へのメール
- 予算アラートは通知のみで課金は止まらないため、下記の割り当て上限と併用する

**割り当て上限** (Google Maps Platform → 割り当て)

| API | 用途 | 上限 | 根拠 |
| --- | --- | --- | --- |
| Maps JavaScript API (マップの読み込み) | フロント | 500 回/日 | 想定は 30〜40 回/日。上限まで使っても超過額は月 $35 程度 |
| Places API (New): Text Search | 登録時の place_id 解決 | 50 回/日 | 月 1,550 回以内で無料枠 (5,000 回) に収まる |
| Places API (New): Place Details | 定期ジョブ | 100 回/日 | 月 3,100 回以内で無料枠 (5,000 回) に収まる |

- 1 日単位の上限が設定できない API は、1 分単位の上限を下げて代用する
- 地図の上限に達すると当日は地図が表示されなくなる。アクセス増加時は可用性と費用を見て見直す

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
3. Actions が `POST /api/admin/spots` を呼ぶ
4. バックエンドが place_id を解決する
   - 一意に決まった場合: 登録し、結果を Issue にコメントしてクローズする
   - 0 件または複数候補の場合: 候補 (名前、住所、place_id) を Issue にコメントして保留する。メンテナが `/approve place_id=<id>` で再実行する
5. 既存の place_id と一致した場合は新規 spot を作らず appearance だけを追加する
