import { useMap, useMapsLibrary } from "@vis.gl/react-google-maps";
import { useEffect, useRef, useState } from "react";
import { useLocationSelection } from "./locationSelectionContext";
import { useMapData } from "./mapDataContext";
import { createCurrentLocationElement } from "./markerElements";

type Position = {
  lat: number;
  lng: number;
  /** Radius of the uncertainty of the position, in metres. */
  accuracy: number;
};

const locationColor = "#1a73e8";

// Shows where the visitor is: a dot, and a circle as large as the accuracy
// of the position.
function CurrentLocationMarker() {
  const map = useMap();
  const markerLibrary = useMapsLibrary("marker");
  const mapsLibrary = useMapsLibrary("maps");
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
    if (!map || !position || hasPannedRef.current) return;
    // Until the spots are loaded, a spot ID from the URL cannot be checked.
    if (selectedId !== null && status === "loading") return;
    hasPannedRef.current = true;
    const hasValidSelection = selectedId !== null && allSpots.some((s) => s.id === selectedId);
    if (!hasValidSelection) map.panTo(position);
  }, [map, position, selectedId, allSpots, status]);

  // Draw the dot and the accuracy circle.
  useEffect(() => {
    if (!map || !markerLibrary || !mapsLibrary || !position) return;
    const marker = new markerLibrary.AdvancedMarkerElement({
      map,
      position,
      content: createCurrentLocationElement(),
      title: "現在地",
      zIndex: 500,
    });
    const circle = new mapsLibrary.Circle({
      map,
      center: position,
      radius: position.accuracy,
      strokeOpacity: 0,
      fillColor: locationColor,
      fillOpacity: 0.15,
      // Clicks pass through to the map and the pins.
      clickable: false,
    });
    return () => {
      marker.map = null;
      circle.setMap(null);
    };
  }, [map, markerLibrary, mapsLibrary, position]);

  return null;
}

export default CurrentLocationMarker;
