import { useEffect, useRef } from "react";
import { useMap } from "react-leaflet";
import { useLocationSelection } from "../spots/locationSelectionContext";
import { useMapData } from "../spots/mapDataContext";
import { useVisits } from "../visits/visitsContext";
import { SpotMarkerLayer } from "./spotMarkerLayer";

// Draws the spots on the map and keeps the map on the selected one.
function SpotMarkers() {
  const map = useMap();
  const { shownSpots, allSpots } = useMapData();
  const { visitedSpotIds } = useVisits();
  const { selectedId, select } = useLocationSelection();
  const layerRef = useRef<SpotMarkerLayer | null>(null);

  // One layer per map.
  useEffect(() => {
    const layer = new SpotMarkerLayer(map, select);
    layerRef.current = layer;
    return () => {
      layer.destroy();
      layerRef.current = null;
    };
  }, [map, select]);

  // The markers follow the search, the filter, the visits and the selection.
  // This effect lists the dependencies of the one above too, so that it runs
  // again, after it, whenever the layer was replaced.
  useEffect(() => {
    const layer = layerRef.current;
    if (!layer) return;
    layer.setSpots(shownSpots);
    layer.update(visitedSpotIds, selectedId);
  }, [map, select, shownSpots, visitedSpotIds, selectedId]);

  // Pan to the selected spot, also when it is hidden by the search or the filter.
  useEffect(() => {
    if (selectedId === null) return;
    const spot = allSpots.find((s) => s.id === selectedId);
    if (spot) map.panTo([spot.location.lat, spot.location.lng]);
  }, [map, selectedId, allSpots]);

  return null;
}

export default SpotMarkers;
