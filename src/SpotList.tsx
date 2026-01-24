import { useEffect, useMemo, useState } from "react";
import { List, useListRef } from "react-window";
import { useLocationSelection } from "./locationSelectionContext";
import { idToIndex, spotsData } from "./mapData";
import SpotItem from "./SpotItem";
import "./SpotList.css";

// Main SpotList component
function SpotList(props: {height: number}) {
  const rowHeight = 300;
  const [query, setQuery] = useState("");
  const listRef = useListRef(null);
  const {selectedId} = useLocationSelection();

  // Scroll to selected item when it changes
  useEffect(()=> {
    if (selectedId === null || !listRef.current) return;
    listRef.current.scrollToRow({
      align: "center",
      behavior: "smooth",
      index: idToIndex(selectedId),
    });
  }, [listRef, selectedId]);

  // Filter features based on search query
  const filtered = useMemo(() => {
    const q = query.trim().toLowerCase();
    if (!q) return spotsData.features;
    return spotsData.features.filter((f) => {
      const p = f.properties;
      return (
        (p.name && p.name.toLowerCase().includes(q)) ||
        (p.address && p.address.toLowerCase().includes(q))
      );
    });
  }, [query]);

  return (
    <div id="spotListComponent" style={{ height: props.height }}>
      <div id="spotListSearch">
        <input
          aria-label="検索"
          placeholder="検索 (名前・住所)"
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
        listRef={listRef}
      />
    </div>
  );
}

export default SpotList;
