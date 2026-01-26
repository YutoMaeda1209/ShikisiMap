import type { Feature, Geometry } from "geojson";
import { useMemo } from "react";
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
  const isSelected = useMemo(() => selectedId === feature.id, [selectedId, feature.id]);

  function onClickSpotItem() {
    if (isClosed) return;
    select(feature.id as string);
  }

  return (
    <div
      className={`spotItem${isClosed ? " closed" : ""}${isSelected ? " selected" : ""}`}
      onClick={onClickSpotItem}
      style={style}
    >
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
  );
}

export default SpotItem;
