import L, { type LatLng } from 'leaflet';
import "leaflet/dist/leaflet.css";
import { useEffect, useState } from 'react';
import { Marker, useMapEvents } from 'react-leaflet';
import currentLocationIcon from "./assets/img/current_location.webp";
import { useLocationSelection } from "./locationSelectionContext";
import { useMapData } from './mapDataContext';

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
  const [isInitMapPan, setIsInitMapPan] = useState<boolean>(false);
  const { selectedId } = useLocationSelection();
  const { allSpotData } = useMapData();
  const hasValidSelection =
    selectedId !== null &&
    allSpotData.features.some((feature) => feature.id === selectedId);

  // Set up map events to track location
  const map = useMapEvents({
    locationfound(event) {
      setPosition(event.latlng);
      if (isInitMapPan || hasValidSelection) return;
      setIsInitMapPan(true);
      map.panTo(event.latlng);
    }
  });

  // Start locating the user when the component mounts
  useEffect(() => {
    map.locate({ watch: true });
    return () => {
      map.stopLocate();
    };
  }, [map]);

  return position === null ? null : (
    <Marker position={position} icon={currentLocationMarkerIcon} />
  );
}

export default CurrentLocationMarker;
