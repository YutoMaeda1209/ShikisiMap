[[日本語 (JA)](/README.md) / English (EN)]

# ShikisiMap

This is a map web app that collects spots introduced in videos by Vtuber Tetora Shikishima (敷嶋てとら) from her YouTube channel: https://www.youtube.com/@ch.1676. It shows spots around the user's current location and supports lightweight YouTube video previews.

**Key Features**
- **Interactive Map**: Built with `leaflet` and `react-leaflet` for a responsive map experience.
- **Spot List**: Browse, filter, and select spots from a sidebar.
- **Current Location Marker**: Displays the user's current location using the browser's geolocation.
- **Lightweight Media Embeds**: Uses `react-lite-youtube-embed` for efficient video previews when needed.

**Tech Stack**
- Framework: React 19 + TypeScript
- Bundler: Vite
- Mapping: Leaflet (`leaflet`, `react-leaflet`)
- Utilities: `react-window` for list virtualization, plus small helpers in `src/utils`

**Quick Start**
1. Install dependencies:

```bash
npm install
```

2. Start the development server:

```bash
npm run dev
```

Open the app at the address printed by Vite (usually `http://localhost:5173`).

**Important Files**
- `src/` — source code
  - `Map.tsx`: map component
  - `Sidebar.tsx`: spot list and selection UI
  - `SpotList.tsx`, `SpotItem.tsx`: list rendering logic
  - `data.json` / `mapData.ts`: spot data
- `docs/spot_guideline.md` — guidelines for adding spot entries

**Contributing**
Contributions are welcome — bug reports, feature ideas, and pull requests. Please follow the repository's contribution guidelines: [CONTRIBUTING.md](.github/CONTRIBUTING.md).

**License**
See the `LICENSE` file for project licensing information.
