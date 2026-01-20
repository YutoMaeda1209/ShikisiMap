import type { Feature, Geometry, Point } from "geojson";
import LiteYouTubeEmbed from 'react-lite-youtube-embed';
import 'react-lite-youtube-embed/dist/LiteYouTubeEmbed.css';
import type { RowComponentProps } from "react-window";
import { useMapControls } from './MapContext';
import "./SpotItem.css";
import type { GeoProperties } from "./types";

interface ListItemProps {
  features: Feature<Geometry, GeoProperties>[];
}

function SpotItem({ index, features, style }: RowComponentProps<ListItemProps>) {
  const feature = features[index];
  const properties = feature.properties;
  const controls = useMapControls();
  const panZoomLevel = 16;
  const isClosed = properties.isClosed;
  const spotId = isClosed ? -1 : index;

  function onClickSpotItem() {
    if (isClosed) return;
    const point: Point = feature.geometry as Point;
    const [lng, lat] = point.coordinates;
    controls.panTo([lat, lng], panZoomLevel);
  }

  return (
    <div id={`spot-${spotId}`} className={`spotItem ${spotId === -1 ? 'closed' : ''}`} onClick={onClickSpotItem} style={style}>
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
