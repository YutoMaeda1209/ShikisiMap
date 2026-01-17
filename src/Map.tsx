import type { Feature, FeatureCollection, GeoJsonObject, Geometry } from 'geojson';
import L, { type LatLng, type Layer } from 'leaflet';
import "leaflet/dist/leaflet.css";
import { useEffect, useState } from 'react';
import { GeoJSON, MapContainer, Marker, TileLayer, useMap, useMapEvents } from 'react-leaflet';
import currentLocationIcon from "./assets/img/current_location.webp";
import geoJsonRowData from "./data.json";
import "./Map.css";
import type { GeoProperties } from './types';

const geoData = geoJsonRowData as GeoJsonObject;

// If this is a FeatureCollection, filter out features where properties.isClosed === true
let features = [] as Feature<Geometry, GeoProperties>[];
const featureCollection = geoData as FeatureCollection<Geometry, GeoProperties>;
features = (featureCollection.features || []).filter((f) => !f.properties?.isClosed) as Feature<Geometry, GeoProperties>[];

// Build a lookup from feature name -> index to match sidebar items (spot-<index>)
const nameToIndex: Record<string, number> = {};
features.forEach((f, i) => {
  nameToIndex[f.properties.name] = i;
});

// Create a filtered FeatureCollection to feed into the map component
const filteredGeoData: FeatureCollection<Geometry, GeoProperties> = {
  ...featureCollection,
  features,
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

function Map({ onMapCreated }: { onMapCreated?: (map: L.Map) => void }) {
  const onEachFeature = (feature: Feature<Geometry, GeoProperties>, layer: Layer) => {
    const idx = nameToIndex[feature.properties.name];
    (layer as L.Evented).on('click', () => {
      const el = document.getElementById(`spot-${idx}`);
      if (el) el.scrollIntoView({ behavior: 'smooth', block: 'start' });
    });
  };

  return (
    <MapContainer center={[35.676423, 139.650027]} zoom={14} zoomControl={false}>
      <TileLayer
        attribution='&copy; <a href="https://www.openstreetmap.org/copyright">OpenStreetMap</a> contributors'
        url="https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png"
      />
      <GeoJSON data={filteredGeoData} onEachFeature={onEachFeature} />
      {onMapCreated ? <MapController onMapCreated={onMapCreated} /> : null}
      <CurrentLocationMarker />
    </MapContainer>
  );
}

export default Map;
