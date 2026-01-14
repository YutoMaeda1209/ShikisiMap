import type { Feature, FeatureCollection, Geometry } from "geojson";
import "./List.css";
import geoJsonRowData from "./data.json";
import type { GeoProperties } from "./types";

function List() {
    const features = (geoJsonRowData as FeatureCollection<Geometry, GeoProperties>).features;
    const listItems = features.map((f: Feature<Geometry, GeoProperties>, i: number) => {
        const properties = f.properties;
        return (
            <li key={i}>
                <h3>{properties.name}</h3>
                <p>{properties.category}</p>
                <p>{properties.address}</p>
                <a href={properties.youtube} target="_blank">YouTube</a>
            </li>
        );
    });

    return (
        <div id="pointList">
            <h2>ShikisiMap</h2>
            <ul>
                {features.length !== 0 ? listItems : <li>データが見つかりません</li>}
            </ul>
        </div>
    );
}

export default List;
