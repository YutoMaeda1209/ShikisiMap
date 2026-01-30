import { useCallback, useEffect, useLayoutEffect, useRef, useState } from "react";
import LiteYouTubeEmbed from "react-lite-youtube-embed";
import "react-lite-youtube-embed/dist/LiteYouTubeEmbed.css";
import { List, useListRef } from "react-window";
import { useLocationSelection } from "./locationSelectionContext";
import { idToIndex, spotsData } from "./mapData";
import SpotItem from "./SpotItem";
import "./SpotList.css";

export type SpotListProps = {
  height: number;
  isCollapsible: boolean;
  isOpen: boolean;
  disableInlineVideo: boolean;
  inlineVideoResetKey: number;
  onRequestOpen: () => void;
  onRequestClose: () => void;
};

// Main SpotList component
const SpotList: React.FC<SpotListProps> = (props) => {
  const [query, setQuery] = useState("");
  const listRef = useListRef(null);
  const { selectedId } = useLocationSelection();
  const scrollTimeoutRef = useRef<number | null>(null);
  const measureRef = useRef<HTMLDivElement | null>(null);
  const [measuredHeight, setMeasuredHeight] = useState<number | null>(null);

  const clearScrollTimeout = useCallback(() => {
    if (scrollTimeoutRef.current !== null) {
      window.clearTimeout(scrollTimeoutRef.current);
      scrollTimeoutRef.current = null;
    }
  }, []);

  const scrollToSelected = useCallback(() => {
    if (selectedId === null || !listRef.current) return;
    listRef.current.scrollToRow({
      align: "center",
      behavior: "smooth",
      index: idToIndex(selectedId),
    });
  }, [listRef, selectedId]);

  useEffect(() => {
    if (!props.isOpen) return;
    clearScrollTimeout();
    const t = window.setTimeout(() => {
      requestAnimationFrame(() => {
        scrollToSelected();
      });
    }, 200);
    scrollTimeoutRef.current = t;
    return () => {
      window.clearTimeout(t);
      scrollTimeoutRef.current = null;
    };
  }, [props.isOpen, selectedId, scrollToSelected, clearScrollTimeout]);

  // Filter features based on search query
  const filtered = (() => {
    const q = query.trim().toLowerCase();
    if (!q) return spotsData.features;
    return spotsData.features.filter((f) => {
      const p = f.properties;
      return (
        (p.name && p.name.toLowerCase().includes(q)) ||
        (p.address && p.address.toLowerCase().includes(q))
      );
    });
  })();

  useLayoutEffect(() => {
    const node = measureRef.current;
    if (!node) return;
    const update = () => {
      const next = Math.ceil(node.scrollHeight);
      setMeasuredHeight((prev) => (prev === next ? prev : next));
    };
    update();
    const ro = new ResizeObserver(update);
    ro.observe(node);
    window.addEventListener("resize", update);
    window.addEventListener("orientationchange", update);
    return () => {
      ro.disconnect();
      window.removeEventListener("resize", update);
      window.removeEventListener("orientationchange", update);
    };
  }, [filtered.length, props.isOpen]);

  const rowHeight = (measuredHeight ?? 240) + 8;

  return (
    <div
      id="spotListComponent"
      style={{ height: props.isOpen ? props.height : "auto" }}
    >
      <div id="spotListSearch">
        <input
          aria-label="検索"
          placeholder="検索 (名前・住所)"
          value={query}
          onChange={(e) => setQuery(e.target.value)}
          onFocus={props.onRequestOpen}
          onClick={props.onRequestOpen}
        />
      </div>
      {filtered[0] ? (
        <div
          className={`spotItem${filtered[0].properties.isClosed ? " closed" : ""}`}
          style={{
            position: "absolute",
            visibility: "hidden",
            pointerEvents: "none",
            width: "100%",
            left: 0,
            top: 0,
          }}
        >
          <div className="spotContent" ref={measureRef}>
            <span className="spotTitle">
              <span>{filtered[0].properties.name}</span>
              {filtered[0].properties.isClosed ? (
                <span className="closedBadge">閉業</span>
              ) : null}
            </span>
            <LiteYouTubeEmbed
              key={`preview-${props.inlineVideoResetKey}`}
              id={filtered[0].properties.youtubeId}
              title={filtered[0].properties.name}
              lazyLoad={true}
              params={`?start=${filtered[0].properties.timestamp}`}
            />
            <div className="address">{filtered[0].properties.address}</div>
          </div>
        </div>
      ) : null}
      <div
        id="spotListPanel"
        className={
          props.isCollapsible ? (props.isOpen ? "open" : "closed") : "open"
        }
        style={{ height: props.isOpen ? props.height : 0 }}
      >
        <List
          id="spotList"
          rowComponent={SpotItem}
          rowCount={filtered.length}
          rowHeight={rowHeight}
          rowProps={{
            features: filtered,
            disableInlineVideo: props.disableInlineVideo,
            inlineVideoResetKey: props.inlineVideoResetKey,
          }}
          listRef={listRef}
        />
        {props.isCollapsible ? (
          <button
            id="spotListClose"
            type="button"
            onClick={props.onRequestClose}
          >
            閉じる
          </button>
        ) : null}
      </div>
    </div>
  );
};

export default SpotList;
