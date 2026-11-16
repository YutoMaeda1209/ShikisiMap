import { APIProvider, Map as GoogleMap } from "@vis.gl/react-google-maps";
import CurrentLocationMarker from "./CurrentLocationMarker";
import "./Map.css";
import MapErrorBoundary from "./MapErrorBoundary";
import SpotMarkers from "./SpotMarkers";

const apiKey = import.meta.env.VITE_GOOGLE_MAPS_API_KEY;
// The demo ID allows advanced markers during development. The production ID
// carries the cloud style that hides Google's own business icons.
const mapId = import.meta.env.VITE_GOOGLE_MAP_ID || "DEMO_MAP_ID";

const defaultCenter = { lat: 35.676423, lng: 139.650027 };

function requestSidebarClose() {
  window.dispatchEvent(new CustomEvent("sidebar:request-close"));
}

function MapView() {
  if (!apiKey) {
    return (
      <div className="mapFallback" onClick={requestSidebarClose}>
        地図を表示できません。VITE_GOOGLE_MAPS_API_KEY が設定されていません。
      </div>
    );
  }

  return (
    <MapErrorBoundary>
      <GoogleMapView apiKey={apiKey} />
    </MapErrorBoundary>
  );
}

function GoogleMapView({ apiKey }: { apiKey: string }) {
  return (
    <APIProvider apiKey={apiKey} language="ja" region="JP">
      <GoogleMap
        className="map"
        mapId={mapId}
        defaultCenter={defaultCenter}
        defaultZoom={14}
        disableDefaultUI
        gestureHandling="greedy"
        // Google's own places are not clickable, so that they are not taken for spots.
        clickableIcons={false}
        onClick={requestSidebarClose}
      >
        <SpotMarkers />
        <CurrentLocationMarker />
      </GoogleMap>
    </APIProvider>
  );
}

export default MapView;
