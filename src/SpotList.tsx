import { useCallback, useEffect, useLayoutEffect, useRef, useState } from "react";
import LiteYouTubeEmbed from "react-lite-youtube-embed";
import "react-lite-youtube-embed/dist/LiteYouTubeEmbed.css";
import { List, useListRef } from "react-window";
import { useLocationSelection } from "./locationSelectionContext";
import { useMapData } from "./mapDataContext";
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
  const listRef = useListRef(null);
  const { selectedId } = useLocationSelection();
  const { listSpotData, idToIndex, filterData } = useMapData();
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
    listRef.current.scrollToRow({
      align: "center",
      behavior: "smooth",
      index: idToIndex(selectedId, listSpotData),
    });
  }, [listRef, selectedId, idToIndex, listSpotData]);

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

  const features = listSpotData.features;

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
  }, [features.length, props.isOpen]);

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
          onChange={(e) => filterData(e.target.value)}
          onFocus={props.onRequestOpen}
          onClick={props.onRequestOpen}
        />
      </div>
      {features[0] ? (
        <div
          className={`spotItem${features[0].properties.isClosed ? " closed" : ""}`}
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
              <span>{features[0].properties.name}</span>
              {features[0].properties.isClosed ? (
                <span className="closedBadge">閉業</span>
              ) : null}
            </span>
            <LiteYouTubeEmbed
              key={`preview-${props.inlineVideoResetKey}`}
              id={features[0].properties.youtubeId}
              title={features[0].properties.name}
              lazyLoad={true}
              params={`?start=${features[0].properties.timestamp}`}
            />
            <div className="address">{features[0].properties.address}</div>
          </div>
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
        <List
          id="spotList"
          rowComponent={SpotItem}
          rowCount={features.length}
          rowHeight={rowHeight}
          rowProps={{
            features: features,
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
