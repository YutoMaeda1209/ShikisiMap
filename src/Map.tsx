import type { Feature, Geometry } from 'geojson';
import L, { type Layer } from 'leaflet';
import markerIcon2x from 'leaflet/dist/images/marker-icon-2x.png';
import markerIcon from 'leaflet/dist/images/marker-icon.png';
import markerShadow from 'leaflet/dist/images/marker-shadow.png';
import "leaflet/dist/leaflet.css";
import { useEffect, useRef } from 'react';
import { GeoJSON, MapContainer, TileLayer } from 'react-leaflet';
import CurrentLocationMarker from './CurrentLocationMarker';
import { useLocationSelection } from './locationSelectionContext';
import "./Map.css";
import { openSpotsData, spotsData, type GeoProperties } from './mapData';

// Main Map component
function Map() {
  const {selectedId, select} = useLocationSelection();
  const mapRef = useRef<L.Map>(null);

  useEffect(() => {
    // Fix Leaflet's default icon paths
    L.Icon.Default.prototype.options.iconUrl = markerIcon;
    L.Icon.Default.prototype.options.iconRetinaUrl = markerIcon2x;
    L.Icon.Default.prototype.options.shadowUrl = markerShadow;
    L.Icon.Default.imagePath = "";

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

  return (
    <MapContainer center={[35.676423, 139.650027]} zoom={14} zoomControl={false} ref={mapRef}>
      <TileLayer
        attribution='&copy; <a href="https://www.openstreetmap.org/copyright">OpenStreetMap</a> contributors'
        url="https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png"
      />
      <GeoJSON data={openSpotsData} onEachFeature={onEachFeature} />
      <CurrentLocationMarker />
    </MapContainer>
  );
}

export default Map;
