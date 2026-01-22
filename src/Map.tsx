import type { Feature, Geometry } from 'geojson';
import L, { type Layer } from 'leaflet';
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

  // Pan map to selected location when selectedId changes
  useEffect(() => {
    if (selectedId === null || !mapRef.current) return;
    const coords = spotsData.features[selectedId].geometry.coordinates;
    mapRef.current.panTo(new L.LatLng(coords[1], coords[0]));
  }, [selectedId, mapRef]);

  // Function to handle feature clicks
  function onEachFeature(feature: Feature<Geometry, GeoProperties>, layer: Layer) {
    (layer as L.Evented).on('click', () => {
      select(feature.id as number);
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
