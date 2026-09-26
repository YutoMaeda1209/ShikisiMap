import {
  useCallback,
  useEffect,
  useLayoutEffect,
  useMemo,
  useRef,
  useState,
} from "react";
import LiteYouTubeEmbed from "react-lite-youtube-embed";
import "react-lite-youtube-embed/dist/LiteYouTubeEmbed.css";
import { List, useListRef } from "react-window";
import { useLocationSelection } from "./locationSelectionContext";
import { useMapData } from "./mapDataContext";
import SpotItem from "./SpotItem";

export type SpotListProps = {
  height: number;
  isCollapsible: boolean;
  isOpen: boolean;
  disableInlineVideo: boolean;
  inlineVideoResetKey: number;
  onRequestOpen: () => void;
  onRequestClose: () => void;
  rowComponent?: typeof SpotItem;
  focusMapOnSelect?: boolean;
};

const previewContentClass =
  "mx-auto flex w-full max-w-[clamp(260px,80vw,360px)] flex-col gap-2.5 py-1.5 [&_.yt-lite]:mx-auto [&_.yt-lite]:w-full [&_.yt-lite]:max-w-[clamp(260px,80vw,360px)] [&_.yt-lite]:overflow-hidden [&_.yt-lite]:rounded-[10px]";

type SortOrder = "newest" | "oldest";

const SpotList: React.FC<SpotListProps> = (props) => {
  const listRef = useListRef(null);
  const { selectedId } = useLocationSelection();
  const { listSpotData, filterData } = useMapData();
  const scrollTimeoutRef = useRef<number | null>(null);
  const measureRef = useRef<HTMLDivElement | null>(null);
  const listPanelRef = useRef<HTMLDivElement | null>(null);
  const [measuredHeight, setMeasuredHeight] = useState<number | null>(null);
  const [sortOrder, setSortOrder] = useState<SortOrder>("newest");
  const searchBoxRef = useRef<HTMLDivElement | null>(null);
  const features = useMemo(
    () =>
      sortOrder === "newest"
        ? [...listSpotData.features].reverse()
        : listSpotData.features,
    [listSpotData.features, sortOrder],
  );

  const clearScrollTimeout = useCallback(() => {
    if (scrollTimeoutRef.current !== null) {
      window.clearTimeout(scrollTimeoutRef.current);
      scrollTimeoutRef.current = null;
    }
  }, []);

  const scrollToSelected = useCallback(() => {
    if (selectedId === null || !listRef.current) return;
    const index = features.findIndex((feature) => feature.id === selectedId);
    if (index === -1) return;
    listRef.current.scrollToRow({
      align: "center",
      behavior: "smooth",
      index,
    });
  }, [listRef, selectedId, features]);

  useEffect(() => {
    if (!props.isOpen) return;
    clearScrollTimeout();
    const timeout = window.setTimeout(() => {
      requestAnimationFrame(scrollToSelected);
    }, 200);
    scrollTimeoutRef.current = timeout;
    return () => {
      window.clearTimeout(timeout);
      scrollTimeoutRef.current = null;
    };
  }, [props.isOpen, selectedId, scrollToSelected, clearScrollTimeout]);

  useLayoutEffect(() => {
    const node = measureRef.current;
    const searchBoxNode = searchBoxRef.current;
    if (!node || !searchBoxNode) return;
    const update = () => {
      const next = Math.ceil(node.scrollHeight);
      setMeasuredHeight((previous) => (previous === next ? previous : next));
      if (!listPanelRef.current) return;
      listPanelRef.current.style.maxHeight = `calc(100% - ${searchBoxNode.scrollHeight}px)`;
    };
    update();
    const observer = new ResizeObserver(update);
    observer.observe(node);
    observer.observe(searchBoxNode);
    window.addEventListener("resize", update);
    window.addEventListener("orientationchange", update);
    return () => {
      observer.disconnect();
      window.removeEventListener("resize", update);
      window.removeEventListener("orientationchange", update);
    };
  }, [features.length, props.isOpen]);

  const rowHeight = (measuredHeight ?? 240) + 8;

  function toggleSortOrder() {
    setSortOrder((current) =>
      current === "newest" ? "oldest" : "newest",
    );
    listRef.current?.scrollToRow({
      index: 0,
      align: "start",
      behavior: "auto",
    });
  }

  return (
    <div style={{ height: props.isOpen ? props.height : "auto" }}>
      <div
        ref={searchBoxRef}
        className="mx-auto box-border w-full max-w-[clamp(260px,80vw,360px)] pb-1.5 flex items-center gap-2"
      >
        <input
          className="box-border min-w-0 flex-1 rounded-lg border border-[#ccc] px-2.5 py-2 text-(length:--font-sm) overflow-y-auto"
          aria-label="検索"
          placeholder="検索 (名前・住所)"
          onChange={(event) => filterData(event.target.value)}
          onFocus={props.onRequestOpen}
          onClick={props.onRequestOpen}
        />
        <button
          type="button"
          className={`shrink-0 whitespace-nowrap rounded-lg border-[#ccc] px-3 py-2 text-(length:--font-sm) font-semibold ${sortOrder === "newest" ? "bg-[#ff6b00]/50 text-white" : "text-[#ff6b00]/50 border" }`}
          aria-label="並び順を切り替える"
          onClick={toggleSortOrder}
        >
          {sortOrder === "newest" ? "新しい順" : "古い順"}
        </button>
      </div>
      {features[0] ? (
        <div className="pointer-events-none invisible absolute top-0 left-0 w-full rounded-[10px] px-3 py-1">
          <div className={previewContentClass} ref={measureRef}>
            <span
              className={`relative flex items-center gap-2 truncate text-(length:--font-xl) leading-[1.1] font-bold ${
                features[0].properties.isClosed ? "pr-18" : ""
              }`}
            >
              <span className="truncate">{features[0].properties.name}</span>
              {features[0].properties.isClosed ? (
                <span className="absolute top-1/2 right-2 w-14 -translate-y-1/2 rounded bg-[#c0392b] px-1.5 py-0.5 text-center text-(length:--font-sm) text-white">
                  閉業
                </span>
              ) : null}
            </span>
            <LiteYouTubeEmbed
              key={`preview-${props.inlineVideoResetKey}`}
              id={features[0].properties.youtubeId}
              title={features[0].properties.name}
              lazyLoad={true}
              params={`?start=${features[0].properties.timestamp}`}
            />
            <div className="w-full truncate">{features[0].properties.address}</div>
          </div>
        </div>
      ) : null}
      <div
        className="overflow-hidden transition-[height] duration-200"
        style={{ height: props.isOpen ? props.height : 0 }}
        ref={listPanelRef}
      >
        <List
          className="flex min-h-0 flex-auto flex-col gap-2 scrollbar-none aria-hidden:shrink-0 [&::-webkit-scrollbar]:hidden"
          rowComponent={props.rowComponent ?? SpotItem}
          rowCount={features.length}
          rowHeight={rowHeight}
          rowProps={{
            features,
            disableInlineVideo: props.disableInlineVideo,
            inlineVideoResetKey: props.inlineVideoResetKey,
            focusMapOnSelect: props.focusMapOnSelect,
          }}
          listRef={listRef}
        />
        {props.isCollapsible ? (
          <button
            className="mt-2 w-full cursor-pointer rounded-md border border-[#ccc] bg-white px-2.5 py-2 text-(length:--font-sm)"
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
