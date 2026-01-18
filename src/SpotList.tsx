import type { FeatureCollection, Geometry } from "geojson";
import { List } from "react-window";
import geoJsonRowData from "./data.json";
import SpotItem from "./SpotItem";
import "./SpotList.css";
import type { GeoProperties } from "./types";

const rowHeight = 320;
function SpotList(props: {height: number}) {
  // Prepare list items from GeoJSON data
  const features = (geoJsonRowData as FeatureCollection<Geometry, GeoProperties>).features;

  return (
    <div id="spotListComponent" style={{ height: props.height }}>
      <List
        id="spotList"
        rowComponent={SpotItem}
        rowCount={features.length}
        rowHeight={rowHeight}
        rowProps={{ features }}
      />
    </div>
  );
}

export default SpotList;
