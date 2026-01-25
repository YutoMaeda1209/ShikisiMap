[日本語 (JA) / [English (EN)](/README-en.md)]

# ShikisiMap

Vtuberの[敷嶋てとら](https://www.youtube.com/@ch.1676)さんが投稿した動画で紹介されたスポットをまとめたマップウェブサイトです。
現在地周辺のスポット表示や、YouTube動画のプレビューもサポートしています。

**主な特徴**
- **地図表示**: `leaflet` と `react-leaflet` によるインタラクティブな地図表示
- **スポット一覧**: サイドバーでスポットを一覧・フィルタ・選択可能
- **現在地マーカー**: ブラウザの位置情報に基づく現在地表示
- **軽量メディア埋め込み**: `react-lite-youtube-embed` を用いた動画プレビュー（必要に応じて）

**技術スタック**
- フレームワーク: React 19, TypeScript
- ビルド: Vite
- 地図: Leaflet (`leaflet`, `react-leaflet`)
- その他: `react-window`（リスト仮想化）など

**クイックスタート**
1. 開発環境のセットアップ（DevContainer）:
1. 開発サーバー起動:
```bash
npm run dev
```

**主なファイル**
- `src/`: ソースコードディレクトリ
    - `Map.tsx`: 地図コンポーネント
    - `Sidebar.tsx`: スポット一覧／選択 UI
    - `SpotList.tsx`, `SpotItem.tsx`: スポット一覧表示ロジック
    - `data.json` / `mapData.ts`: スポットデータ
- `docs/spot_guideline.md`: スポットの記載ガイドライン

**コントリビューション**
コントリビューションは大歓迎です！バグ報告、機能提案、プルリクエストなど、お気軽にどうぞ。
コントリビューション時は[コントリビューションガイドライン](/.github/CONTRIBUTING.md)を参照してください。

**ライセンス**
[ライセンスファイル](LICENSE)を参照してください。
