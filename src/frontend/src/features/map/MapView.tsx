import "leaflet/dist/leaflet.css";
import { MapContainer, TileLayer, useMapEvents } from "react-leaflet";
import CurrentLocationMarker from "./CurrentLocationMarker";
import "./MapView.css";
import SpotMarkers from "./SpotMarkers";

const defaultCenter: [number, number] = [35.676423, 139.650027];

// A click on the map itself, not on a marker, closes the sidebar on narrow screens.
function MapClickCloser() {
  useMapEvents({
    click: () => {
      window.dispatchEvent(new CustomEvent("sidebar:request-close"));
    },
  });
  return null;
}

function MapView() {
  return (
    <MapContainer className="map" center={defaultCenter} zoom={14} zoomControl={false}>
      <TileLayer
        attribution='&copy; <a href="https://www.openstreetmap.org/copyright">OpenStreetMap</a> contributors'
        url="https://tile.openstreetmap.org/{z}/{x}/{y}.png"
        maxZoom={19}
      />
      <SpotMarkers />
      <CurrentLocationMarker />
      <MapClickCloser />
    </MapContainer>
  );
}

export default MapView;
