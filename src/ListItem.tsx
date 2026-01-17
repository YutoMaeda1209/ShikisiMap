import type { Feature, Geometry, Point } from "geojson";
import "./ListItem.css";
import { useMapControls } from './MapContext';
import type { GeoProperties } from "./types";

interface ListItemProps {
  feature: Feature<Geometry, GeoProperties>;
  index: number;
}

function ListItem({ feature, index }: ListItemProps) {
  const properties = feature.properties;
  const controls = useMapControls();
  const panZoomLevel = 13;
  const isClosed = Boolean(properties?.isClosed);

  function onClickSpotItem() {
    if (isClosed) return;
    const point: Point = feature.geometry as Point;
    const [lng, lat] = point.coordinates;
    controls.panTo([lat, lng], panZoomLevel);
  }

  return (
    <li id={`spot-${index}`} className={`spotItem ${isClosed ? 'closed' : ''}`} key={index} onClick={onClickSpotItem}>
      <span className="spotTitle">
        <span>{properties.name}</span>
        {isClosed && <span className="closedBadge">閉業</span>}
      </span>
      <iframe
        className={`previewVideo ${isClosed ? 'closedIframe' : ''}`}
        width="100%"
        src={`https://www.youtube-nocookie.com/embed/${properties.youtubeId}?start=${properties.timestamp}`}
        title="YouTube video player"
        allow="accelerometer; autoplay; clipboard-write; encrypted-media; gyroscope; picture-in-picture; web-share"
        referrerPolicy="strict-origin-when-cross-origin"
        allowFullScreen
      ></iframe>
      <a
        href={`https://www.google.com/maps/search/${properties.name} ${properties.address}`}
        target="_blank"
        rel="noopener noreferrer"
      >
        {properties.address}
      </a>
    </li>
  );
}

export default ListItem;
