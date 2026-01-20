import type { FeatureCollection, Geometry } from "geojson";
import { useMemo, useState } from "react";
import { List } from "react-window";
import { rowHeight } from "./App";
import geoJsonRowData from "./data.json";
import SpotItem from "./SpotItem";
import "./SpotList.css";
import type { GeoProperties } from "./types";

function SpotList(props: {height: number}) {
  // Prepare list items from GeoJSON data
  const features = (geoJsonRowData as FeatureCollection<Geometry, GeoProperties>).features;

  const [query, setQuery] = useState("");

  const filtered = useMemo(() => {
    const q = query.trim().toLowerCase();
    if (!q) return features;
    return features.filter((f) => {
      const p = f.properties;
      return (
        (p.name && p.name.toLowerCase().includes(q)) ||
        (p.category && p.category.toLowerCase().includes(q)) ||
        (p.address && p.address.toLowerCase().includes(q))
      );
    });
  }, [query, features]);

  return (
    <div id="spotListComponent" style={{ height: props.height }}>
      <div id="spotListSearch">
        <input
          aria-label="検索"
          placeholder="検索 (名前・カテゴリ・住所)"
          value={query}
          onChange={(e) => setQuery(e.target.value)}
        />
      </div>
      <List
        id="spotList"
        rowComponent={SpotItem}
        rowCount={filtered.length}
        rowHeight={rowHeight}
        rowProps={{ features: filtered }}
      />
    </div>
  );
}

export default SpotList;
