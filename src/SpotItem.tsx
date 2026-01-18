import type { Feature, Geometry, Point } from "geojson";
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
      <iframe
        className="previewVideo"
        width="100%"
        src={`https://www.youtube-nocookie.com/embed/${properties.youtubeId}?start=${properties.timestamp}`}
        title="YouTube video player"
        allow="accelerometer; autoplay; clipboard-write; encrypted-media; gyroscope; picture-in-picture; web-share; compute-pressure;"
        referrerPolicy="strict-origin-when-cross-origin"
        allowFullScreen={true}
      ></iframe>
      <a
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
