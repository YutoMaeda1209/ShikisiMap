import L from "leaflet";
import { useEffect, useRef, useState } from "react";
import { Circle, Marker, useMap } from "react-leaflet";
import { useLocationSelection } from "../spots/locationSelectionContext";
import { useMapData } from "../spots/mapDataContext";

type Position = {
  lat: number;
  lng: number;
  /** Radius of the uncertainty of the position, in metres. */
  accuracy: number;
};

const locationColor = "#1a73e8";

// A 16 px dot with a 3 px white border; the style is in MapView.css.
const locationIcon = L.divIcon({ className: "currentLocation", iconSize: [22, 22] });

// Shows where the visitor is: a dot, and a circle as large as the accuracy
// of the position.
function CurrentLocationMarker() {
  const map = useMap();
  const { selectedId } = useLocationSelection();
  const { allSpots, status } = useMapData();
  const [position, setPosition] = useState<Position | null>(null);
  const hasPannedRef = useRef(false);

  // Follow the visitor's position.
  useEffect(() => {
    if (!("geolocation" in navigator)) return;
    const watchId = navigator.geolocation.watchPosition(
      (pos) => {
        setPosition({
          lat: pos.coords.latitude,
          lng: pos.coords.longitude,
          accuracy: pos.coords.accuracy,
        });
      },
      () => {
        // Denied or unavailable: the map simply has no current location.
      },
      { enableHighAccuracy: true }
    );
    return () => navigator.geolocation.clearWatch(watchId);
  }, []);

  // Move the map to the first position, unless the page was opened on a spot.
  useEffect(() => {
    if (!position || hasPannedRef.current) return;
    // Until the spots are loaded, a spot ID from the URL cannot be checked.
    if (selectedId !== null && status === "loading") return;
    hasPannedRef.current = true;
    const hasValidSelection = selectedId !== null && allSpots.some((s) => s.id === selectedId);
    if (!hasValidSelection) map.panTo([position.lat, position.lng]);
  }, [map, position, selectedId, allSpots, status]);

  if (!position) return null;
  const center: [number, number] = [position.lat, position.lng];
  return (
    <>
      {/* Clicks pass through both to the map and the pins. */}
      <Circle
        center={center}
        radius={position.accuracy}
        interactive={false}
        pathOptions={{ stroke: false, fillColor: locationColor, fillOpacity: 0.15 }}
      />
      <Marker position={center} icon={locationIcon} interactive={false} keyboard={false} zIndexOffset={50000} />
    </>
  );
}

export default CurrentLocationMarker;
