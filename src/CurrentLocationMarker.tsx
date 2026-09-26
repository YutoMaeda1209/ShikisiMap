import * as maplibregl from "maplibre-gl";
import type { Map as MapLibreMap } from "maplibre-gl";
import { useEffect, useRef } from "react";
import currentLocationIcon from "./assets/img/current_location.webp";
import { useLocationSelection } from "./locationSelectionContext";
import { useMapData } from "./mapDataContext";

type CurrentLocationMarkerProps = {
  map: MapLibreMap | null;
};

function CurrentLocationMarker({ map }: CurrentLocationMarkerProps) {
  const { selectedId } = useLocationSelection();
  const { allSpotData } = useMapData();
  const hasInitialMapPanRef = useRef(false);
  const hasValidSelectionRef = useRef(false);

  useEffect(() => {
    hasValidSelectionRef.current =
      selectedId !== null &&
      allSpotData.features.some((feature) => feature.id === selectedId);
  }, [selectedId, allSpotData]);

  useEffect(() => {
    if (!map || !("geolocation" in navigator)) return;

    const element = document.createElement("img");
    element.className = "h-8 w-8";
    element.src = currentLocationIcon;
    element.alt = "Current location";
    const marker = new maplibregl.Marker({ element });
    let markerAdded = false;

    const watchId = navigator.geolocation.watchPosition((position) => {
      const coordinates: [number, number] = [
        position.coords.longitude,
        position.coords.latitude,
      ];

      marker.setLngLat(coordinates);
      if (!markerAdded) {
        marker.addTo(map);
        markerAdded = true;
      }

      if (hasInitialMapPanRef.current || hasValidSelectionRef.current) return;
      hasInitialMapPanRef.current = true;
      map.panTo(coordinates);
    });

    return () => {
      navigator.geolocation.clearWatch(watchId);
      marker.remove();
    };
  }, [map]);

  return null;
}

export default CurrentLocationMarker;
