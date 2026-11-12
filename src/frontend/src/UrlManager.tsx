import { useEffect, useRef } from "react";
import { useLocationSelection } from "./locationSelectionContext";

function UrlManager() {
  const {selectedId, select} = useLocationSelection();
  const hasInitialized = useRef(false);

  // On mount, read the URL parameter and select the spot if valid
  useEffect(() => {
    if (hasInitialized.current) return;
    hasInitialized.current = true;
    if (selectedId !== null) return;
    const params = new URLSearchParams(window.location.search);
    const spotId = params.get('spotId');
    if (!spotId) return;
    select(spotId);
  }, [select, selectedId]);

  // Whenever the selectedId changes, update the URL parameter
  useEffect(() => {
    const params = new URLSearchParams(window.location.search);
    if (selectedId === null) {
      params.delete('spotId');
    } else {
      params.set('spotId', selectedId);
    }
    const newUrl = `${window.location.pathname}?${params.toString()}`;
    window.history.replaceState({}, '', newUrl);
  }, [selectedId]);

  return null;
}

export default UrlManager;
