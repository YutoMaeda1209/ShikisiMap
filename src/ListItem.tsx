import type { Feature, Geometry } from "geojson";
import "./ListItem.css";
import type { GeoProperties } from "./types";

interface ListItemProps {
  feature: Feature<Geometry, GeoProperties>;
  index: number;
}

function ListItem({ feature, index }: ListItemProps) {
  const properties = feature.properties;
  return (
    <li id={`spot-${index}`} className="spotItem" key={index}>
      <h3>{properties.name}</h3>
      <iframe
        className="previewVideo"
        width="100%"
        src={
          "https://www.youtube-nocookie.com/embed/" +
          properties.youtubeId +
          "?start=" + parseInt(properties.timestamp)
        }
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
