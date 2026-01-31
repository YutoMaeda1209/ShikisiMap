import type { Feature, Geometry } from "geojson";
import { useEffect, useLayoutEffect, useRef, useState } from "react";
import LiteYouTubeEmbed from 'react-lite-youtube-embed';
import 'react-lite-youtube-embed/dist/LiteYouTubeEmbed.css';
import type { RowComponentProps } from "react-window";
import { useLocationSelection } from "./locationSelectionContext";
import type { GeoProperties } from "./mapDataContext";
import "./SpotItem.css";

interface ListItemProps {
  features: Feature<Geometry, GeoProperties>[];
  disableInlineVideo?: boolean;
  inlineVideoResetKey?: number;
}

function SpotItem({
  index,
  style,
  features,
  disableInlineVideo = false,
  inlineVideoResetKey = 0,
}: RowComponentProps<ListItemProps>) {
  const feature = features[index];
  const properties = feature.properties;
  const isClosed = properties.isClosed;
  const {selectedId, select} = useLocationSelection();
  const spotItemRootRef = useRef<HTMLDivElement | null>(null);
  const [isCompact, setIsCompact] = useState(false);

  useEffect(() => {
    if (selectedId !== feature.id || !spotItemRootRef.current) return;
    const spotItemRef = spotItemRootRef.current;
    spotItemRef.classList.add("blink-highlight");
    const t = setTimeout(() => spotItemRef.classList.remove("blink-highlight"), 1500);
    return () => {
      spotItemRef.classList.remove("blink-highlight");
      clearTimeout(t);
    };
  }, [selectedId, feature.id]);
  const isSelected = selectedId === feature.id;

  useLayoutEffect(() => {
    const node = spotItemRootRef.current;
    if (!node) return;
    const targetHeight = typeof style?.height === "number" ? style.height : null;
    if (targetHeight === null) return;
    let raf = 0;
    raf = requestAnimationFrame(() => {
      const isOverflowing = node.scrollHeight > targetHeight;
      setIsCompact((prev) => (prev !== isOverflowing ? isOverflowing : prev));
    });
    return () => {
      if (raf) cancelAnimationFrame(raf);
    };
  }, [style?.height, feature.id]);

  function onClickSpotItem() {
    if (isClosed) return;
    if (selectedId === feature.id) {
      window.dispatchEvent(new CustomEvent("sidebar:request-open-sheet"));
      return;
    }
    select(feature.id as string);
  }

  return (
    <div className="spotItemRow" style={style}>
      <div
        ref={spotItemRootRef}
        className={`spotItem${isClosed ? " closed" : ""}${isSelected ? " selected" : ""}${isCompact ? " compact-title" : ""}`}
        onClick={onClickSpotItem}
      >
        <div className="spotContent">
          <span className="spotTitle">
            <span>{properties.name}</span>
            {isClosed ? <span className="closedBadge">閉業</span> : null}
          </span>
          <div className={`ytLiteWrapper${disableInlineVideo ? " disabled" : ""}`}>
            <LiteYouTubeEmbed
              key={`item-${feature.id}-${inlineVideoResetKey}`}
              id={properties.youtubeId}
              title={properties.name}
              lazyLoad={true}
              params={`?start=${properties.timestamp}`}
            />
          </div>
          <a
            className="address"
            href={`https://www.google.com/maps/search/${properties.name} ${properties.address}`}
            target="_blank"
            rel="noopener noreferrer"
          >
            {properties.address}
          </a>
        </div>
      </div>
    </div>
  );
}

export default SpotItem;
