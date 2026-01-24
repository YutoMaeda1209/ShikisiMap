import { useEffect } from "react";
import { useLocationSelection } from "./locationSelectionContext";

function UrlManager() {
  const {selectedId, select} = useLocationSelection();

  // On mount, read the URL parameter and select the spot if valid
  useEffect(() => {
    if (selectedId !== null) return;
    const params = new URLSearchParams(window.location.search);
    const spotId = params.get('spotId');
    if (!spotId) return;
    select(spotId);
  }, [select, selectedId]);

  // Whenever the selectedId changes, update the URL parameter
  useEffect(() => {
    if (selectedId === null) return;
    const params = new URLSearchParams(window.location.search);
    params.set('spotId', selectedId);
    const newUrl = `${window.location.pathname}?${params.toString()}`;
    window.history.replaceState({}, '', newUrl);
  }, [selectedId]);

  return null;
}

export default UrlManager;
