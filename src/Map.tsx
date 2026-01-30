import type { Feature, Geometry } from 'geojson';
import L, { type Layer } from 'leaflet';
import markerIcon2x from 'leaflet/dist/images/marker-icon-2x.png';
import markerIcon from 'leaflet/dist/images/marker-icon.png';
import markerShadow from 'leaflet/dist/images/marker-shadow.png';
import "leaflet/dist/leaflet.css";
import { useEffect, useRef } from 'react';
import { GeoJSON, MapContainer, TileLayer, useMapEvents } from 'react-leaflet';
import CurrentLocationMarker from './CurrentLocationMarker';
import { useLocationSelection } from './locationSelectionContext';
import "./Map.css";
import { openSpotsData, spotsData, type GeoProperties } from './mapData';

const defaultIcon = L.icon({
  iconUrl: markerIcon,
  iconRetinaUrl: markerIcon2x,
  shadowUrl: markerShadow,
  iconSize: [25, 41],
  iconAnchor: [12, 41],
  shadowSize: [41, 41],
  shadowAnchor: [12, 41],
  popupAnchor: [1, -34],
});
const selectedIcon = L.icon({
  iconUrl: markerIcon,
  iconRetinaUrl: markerIcon2x,
  shadowUrl: markerShadow,
  iconSize: [29, 47],
  iconAnchor: [14, 47],
  shadowSize: [47, 47],
  shadowAnchor: [14, 47],
  popupAnchor: [1, -38],
  className: "selected-marker",
});

// Main Map component
function Map() {
  const {selectedId, select} = useLocationSelection();
  const mapRef = useRef<L.Map>(null);

  useEffect(() => {
    // Pan map to selected location when selectedId changes
    if (selectedId === null || !mapRef.current) return;
    const feature = spotsData.features.find(feature => feature.id === selectedId);
    if (!feature) return;
    const coords = feature.geometry.coordinates;
    mapRef.current.panTo(new L.LatLng(coords[1], coords[0]));
  }, [selectedId, mapRef]);

  // Function to handle feature clicks
  function onEachFeature(feature: Feature<Geometry, GeoProperties>, layer: Layer) {
    (layer as L.Evented).on('click', () => {
      select(feature.id as string);
    });
  }

  function pointToLayer(feature: Feature<Geometry, GeoProperties>, latlng: L.LatLng) {
    const isSelected = feature.id === selectedId;
    return L.marker(latlng, {
      icon: isSelected ? selectedIcon : defaultIcon,
    });
  }

  function MapClickCloser() {
    useMapEvents({
      click: () => {
        window.dispatchEvent(new CustomEvent("sidebar:request-close"));
      },
    });
    return null;
  }

  return (
    <MapContainer center={[35.676423, 139.650027]} zoom={14} zoomControl={false} ref={mapRef}>
      <TileLayer
        attribution='&copy; <a href="https://www.openstreetmap.org/copyright">OpenStreetMap</a> contributors'
        url="https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png"
      />
      <GeoJSON
        key={selectedId ?? "none"}
        data={openSpotsData}
        onEachFeature={onEachFeature}
        pointToLayer={pointToLayer}
      />
      <CurrentLocationMarker />
      <MapClickCloser />
    </MapContainer>
  );
}

export default Map;
