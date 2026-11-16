/// <reference types="vite/client" />

interface ImportMetaEnv {
  /** Browser key for Maps JavaScript API. The map is not shown without it. */
  readonly VITE_GOOGLE_MAPS_API_KEY?: string;
  /** Map ID whose cloud style hides the business POIs. Defaults to DEMO_MAP_ID. */
  readonly VITE_GOOGLE_MAP_ID?: string;
}

interface ImportMeta {
  readonly env: ImportMetaEnv;
}
