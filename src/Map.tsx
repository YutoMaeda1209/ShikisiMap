import type { Feature, GeoJsonObject, Geometry } from 'geojson';
import type { Layer } from 'leaflet';
import "leaflet/dist/leaflet.css";
import { GeoJSON, MapContainer, TileLayer } from 'react-leaflet';
import geoJsonRowData from "./data.json";
import "./Map.css";

type GeoProperties = {
    popupContent: string;
}

const geoData = geoJsonRowData as GeoJsonObject;

function Map() {
    const onEachFeature = (feature: Feature<Geometry, GeoProperties>, layer: Layer) => {
        layer.bindPopup(feature.properties.popupContent);
    };

    return (
        <MapContainer center={[35.676423, 139.650027]} zoom={7} zoomControl={false}>
            <TileLayer
                attribution='&copy; <a href="https://www.openstreetmap.org/copyright">OpenStreetMap</a> contributors'
                url="https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png"
            />
            <GeoJSON data={geoData} onEachFeature={onEachFeature} />
        </MapContainer>
    );
}

export default Map;
