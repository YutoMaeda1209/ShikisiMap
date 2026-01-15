import type { Feature, GeoJsonObject, Geometry } from 'geojson';
import type { LatLng, Layer } from 'leaflet';
import "leaflet/dist/leaflet.css";
import { useEffect, useState } from 'react';
import { GeoJSON, MapContainer, Marker, TileLayer, useMapEvents } from 'react-leaflet';
import geoJsonRowData from "./data.json";
import "./Map.css";
import type { GeoProperties } from './types';

const geoData = geoJsonRowData as GeoJsonObject;

function LocationMarker() {
  const [position, setPosition] = useState<LatLng | null>(null);
  const map = useMapEvents({
    locationfound(event) {
      setPosition(event.latlng)
      map.flyTo(event.latlng, map.getZoom())
    },
    locationerror(err) {
      alert(err.message);
    },
  });

  useEffect(() => {
    map.locate()
  }, [map]);

  return position === null ? null : (
    <Marker position={position} />
  );
}

function Map() {
  const onEachFeature = (feature: Feature<Geometry, GeoProperties>, layer: Layer) => {
    layer.bindPopup(feature.properties.name);
  };

  return (
    <MapContainer center={[35.676423, 139.650027]} zoom={14} zoomControl={false}>
      <TileLayer
        attribution='&copy; <a href="https://www.openstreetmap.org/copyright">OpenStreetMap</a> contributors'
        url="https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png"
      />
      <GeoJSON data={geoData} onEachFeature={onEachFeature} />
      <LocationMarker />
    </MapContainer>
  );
}

export default Map;
