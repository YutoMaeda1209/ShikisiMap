import type { Feature, Geometry } from "geojson";
import { useEffect, useLayoutEffect, useMemo, useRef, useState } from "react";
import LiteYouTubeEmbed from 'react-lite-youtube-embed';
import 'react-lite-youtube-embed/dist/LiteYouTubeEmbed.css';
import type { RowComponentProps } from "react-window";
import { useLocationSelection } from "./locationSelectionContext";
import type { GeoProperties } from "./mapData";
import "./SpotItem.css";

interface ListItemProps {
  features: Feature<Geometry, GeoProperties>[];
}

function SpotItem({ index, features, style }: RowComponentProps<ListItemProps>) {
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
  const isSelected = useMemo(() => selectedId === feature.id, [selectedId, feature.id]);

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
    select(feature.id as string);
  }

  return (
    <div
      ref={spotItemRootRef}
      className={`spotItem${isClosed ? " closed" : ""}${isSelected ? " selected" : ""}${isCompact ? " compact-title" : ""}`}
      onClick={onClickSpotItem}
      style={style}
    >
      <div className="spotContent">
        <span className="spotTitle">
          <span>{properties.name}</span>
          {isClosed ? <span className="closedBadge">閉業</span> : null}
        </span>
        <LiteYouTubeEmbed
          id={properties.youtubeId}
          title={properties.name}
          lazyLoad={true}
          params={`?start=${properties.timestamp}`}
        />
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
  );
}

export default SpotItem;
