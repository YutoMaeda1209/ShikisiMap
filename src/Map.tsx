import type { Feature, FeatureCollection, Geometry } from 'geojson';
import L, { type LatLng, type Layer } from 'leaflet';
import "leaflet/dist/leaflet.css";
import { useEffect, useState } from 'react';
import { GeoJSON, MapContainer, Marker, TileLayer, useMap, useMapEvents } from 'react-leaflet';
import currentLocationIcon from "./assets/img/current_location.webp";
import geoJsonRowData from "./data.json";
import "./Map.css";
import type { GeoProperties } from './types';

// Build a lookup from feature name -> index to match sidebar items (spot-<index>)
let geoData = geoJsonRowData as FeatureCollection<Geometry, GeoProperties>;
const features = geoData.features;
const nameToIndex: Record<string, number> = {};
features.forEach((f, i) => {
  nameToIndex[f.properties.name] = i;
});
geoData = {
  type: "FeatureCollection",
  features: features.filter(f => !f.properties.isClosed)
};

// Define custom icon for current location marker
const currentLocationMarkerIcon = new L.Icon({
  iconUrl: currentLocationIcon,
  iconSize: [32, 32],
  iconAnchor: [16, 16],
  popupAnchor: [0, -16],
});

// Component to handle user's current location
function CurrentLocationMarker() {
  const [position, setPosition] = useState<LatLng | null>(null);
  const map = useMapEvents({
    locationfound(event) {
      setPosition(event.latlng);
      map.panTo(event.latlng);
    }
  });

  useEffect(() => {
    map.locate();
  }, [map]);

  return position === null ? null : (
    <Marker position={position} icon={currentLocationMarkerIcon} />
  );
}

function MapController({ onMapCreated }: { onMapCreated: (map: L.Map) => void }) {
  const map = useMap();
  useEffect(() => {
    onMapCreated(map);
  }, [map, onMapCreated]);
  return null;
}

const rowHeight = 320;
function Map({ onMapCreated }: { onMapCreated?: (map: L.Map) => void }) {
  const onEachFeature = (feature: Feature<Geometry, GeoProperties>, layer: Layer) => {
    const idx = nameToIndex[feature.properties.name];
    (layer as L.Evented).on('click', () => {
      const listEl = document.getElementById('spotList') as HTMLElement;
      listEl.scrollTo({ top: idx * rowHeight, behavior: 'smooth' });
    });
  };

  return (
    <MapContainer center={[35.676423, 139.650027]} zoom={14} zoomControl={false}>
      <TileLayer
        attribution='&copy; <a href="https://www.openstreetmap.org/copyright">OpenStreetMap</a> contributors'
        url="https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png"
      />
      <GeoJSON data={geoData} onEachFeature={onEachFeature} />
      {onMapCreated ? <MapController onMapCreated={onMapCreated} /> : null}
      <CurrentLocationMarker />
    </MapContainer>
  );
}

export default Map;
