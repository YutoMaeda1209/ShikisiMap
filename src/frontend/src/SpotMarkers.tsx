import { useMap, useMapsLibrary } from "@vis.gl/react-google-maps";
import { useEffect, useRef } from "react";
import { useLocationSelection } from "./locationSelectionContext";
import { useMapData } from "./mapDataContext";
import { SpotMarkerLayer } from "./spotMarkerLayer";
import { useVisits } from "./visitsContext";

// Draws the spots on the map and keeps the map on the selected one.
function SpotMarkers() {
  const map = useMap();
  const markerLibrary = useMapsLibrary("marker");
  const { shownSpots, allSpots } = useMapData();
  const { visitedSpotIds } = useVisits();
  const { selectedId, select } = useLocationSelection();
  const layerRef = useRef<SpotMarkerLayer | null>(null);

  // One layer per map.
  useEffect(() => {
    if (!map || !markerLibrary) return;
    const layer = new SpotMarkerLayer(map, markerLibrary, select);
    layerRef.current = layer;
    return () => {
      layer.destroy();
      layerRef.current = null;
    };
  }, [map, markerLibrary, select]);

  // The markers follow the search, the filter, the visits and the selection.
  // This effect lists the dependencies of the one above too, so that it runs
  // again, after it, whenever the layer was replaced.
  useEffect(() => {
    const layer = layerRef.current;
    if (!map || !markerLibrary || !layer) return;
    layer.setSpots(shownSpots);
    layer.update(visitedSpotIds, selectedId);
  }, [map, markerLibrary, select, shownSpots, visitedSpotIds, selectedId]);

  // Pan to the selected spot, also when it is hidden by the search or the filter.
  useEffect(() => {
    if (!map || selectedId === null) return;
    const spot = allSpots.find((s) => s.id === selectedId);
    if (spot) map.panTo(spot.location);
  }, [map, selectedId, allSpots]);

  return null;
}

export default SpotMarkers;
