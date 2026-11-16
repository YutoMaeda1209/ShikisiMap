import { useCallback, useEffect, useLayoutEffect, useRef, useState } from "react";
import { List, useListRef } from "react-window";
import { useLocationSelection } from "./locationSelectionContext";
import { useMapData, type VisitFilter } from "./mapDataContext";
import SpotCard from "./SpotCard";
import SpotItem from "./SpotItem";
import "./SpotList.css";
import { useVisits } from "./visitsContext";

const visitFilters: { value: VisitFilter; label: string }[] = [
  { value: "all", label: "すべて" },
  { value: "visited", label: "訪問済み" },
  { value: "unvisited", label: "未訪問" },
];

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
  const listRef = useListRef(null);
  const { selectedId } = useLocationSelection();
  const { shownSpots, status, reload, setQuery, visitFilter, setVisitFilter } = useMapData();
  const { readOnly } = useVisits();
  const scrollTimeoutRef = useRef<number | null>(null);
  const measureRef = useRef<HTMLDivElement | null>(null);
  const listPanelRef = useRef<HTMLDivElement | null>(null);
  const [measuredHeight, setMeasuredHeight] = useState<number | null>(null);
  const searchBoxRef = useRef<HTMLDivElement | null>(null);
  const clearScrollTimeout = useCallback(() => {
    if (scrollTimeoutRef.current !== null) {
      window.clearTimeout(scrollTimeoutRef.current);
      scrollTimeoutRef.current = null;
    }
  }, []);

  const scrollToSelected = useCallback(() => {
    if (selectedId === null || !listRef.current) return;
    // The selected spot may be hidden by the search or the filter.
    const index = shownSpots.findIndex((s) => s.id === selectedId);
    if (index === -1) return;
    listRef.current.scrollToRow({ align: "center", behavior: "smooth", index });
  }, [listRef, selectedId, shownSpots]);

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

  const spots = shownSpots;

  useLayoutEffect(() => {
    const node = measureRef.current;
    const searchBoxNode = searchBoxRef.current;
    if (!node || !searchBoxNode) return;
    const update = () => {
      const next = Math.ceil(node.scrollHeight);
      setMeasuredHeight((prev) => (prev === next ? prev : next));
      if (!listPanelRef.current) return;
      listPanelRef.current.style.maxHeight = `calc(100% - ${searchBoxNode.scrollHeight}px)`;
    };
    update();
    const ro = new ResizeObserver(update);
    ro.observe(node);
    ro.observe(searchBoxNode);
    window.addEventListener("resize", update);
    window.addEventListener("orientationchange", update);
    return () => {
      ro.disconnect();
      window.removeEventListener("resize", update);
      window.removeEventListener("orientationchange", update);
    };
  }, [spots.length, props.isOpen, readOnly]);

  const rowHeight = (measuredHeight ?? 240) + 8;

  return (
    <div
      id="spotListComponent"
      style={{ height: props.isOpen ? props.height : "auto" }}
    >
      <div id="spotListSearch" ref={searchBoxRef}>
        <input
          aria-label="検索"
          placeholder="検索 (名前・住所)"
          onChange={(e) => setQuery(e.target.value)}
          onFocus={props.onRequestOpen}
          onClick={props.onRequestOpen}
        />
        {readOnly ? null : (
          <div id="visitFilter" role="group" aria-label="訪問状態で絞り込み">
            {visitFilters.map((f) => (
              <button
                key={f.value}
                type="button"
                aria-pressed={visitFilter === f.value}
                onClick={() => {
                  setVisitFilter(f.value);
                  props.onRequestOpen();
                }}
              >
                {f.label}
              </button>
            ))}
          </div>
        )}
      </div>
      {/* A hidden copy of the first card gives the height of every row. */}
      {spots[0] ? (
        <div
          className={`spotItem${spots[0].isClosed ? " closed" : ""}`}
          aria-hidden="true"
          inert
          style={{
            position: "absolute",
            visibility: "hidden",
            pointerEvents: "none",
            width: "100%",
            left: 0,
            top: 0,
          }}
        >
          <SpotCard
            spot={spots[0]}
            videoKey={`preview-${props.inlineVideoResetKey}`}
            contentRef={measureRef}
          />
        </div>
      ) : null}
      <div
        id="spotListPanel"
        className={
          props.isCollapsible ? (props.isOpen ? "open" : "closed") : "open"
        }
        style={{ height: props.isOpen ? props.height : 0 }}
        ref={listPanelRef}
      >
        {spots.length > 0 ? (
          <List
            id="spotList"
            rowComponent={SpotItem}
            rowCount={spots.length}
            rowHeight={rowHeight}
            rowProps={{
              spots,
              disableInlineVideo: props.disableInlineVideo,
              inlineVideoResetKey: props.inlineVideoResetKey,
            }}
            listRef={listRef}
          />
        ) : (
          <div id="spotListEmpty" role="status">
            {status === "loading" ? (
              "読み込み中…"
            ) : status === "error" ? (
              <>
                <span>スポットを読み込めませんでした。</span>
                <button type="button" onClick={reload}>
                  再読み込み
                </button>
              </>
            ) : (
              "該当するスポットがありません。"
            )}
          </div>
        )}
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
