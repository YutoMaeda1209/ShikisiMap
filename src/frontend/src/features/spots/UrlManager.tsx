import { useEffect, useRef } from "react";
import { useLocationSelection } from "./locationSelectionContext";

// Keeps the URL and the page in sync: the selected spot lives in the
// "spotId" parameter, and one-off parameters are consumed on load. The path
// (the map or /users/:id) is left as it is.
function UrlManager() {
  const {selectedId, select} = useLocationSelection();
  const hasInitialized = useRef(false);

  // On mount, read the URL parameters
  useEffect(() => {
    if (hasInitialized.current) return;
    hasInitialized.current = true;

    const params = new URLSearchParams(window.location.search);

    // The login callback redirects here with this flag when it failed.
    if (params.get("login") === "failed") {
      params.delete("login");
      const query = params.toString();
      window.history.replaceState({}, "", `${window.location.pathname}${query ? `?${query}` : ""}`);
      window.dispatchEvent(
        new CustomEvent("notice:show", { detail: "ログインできませんでした。もう一度お試しください。" })
      );
    }

    if (selectedId !== null) return;
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
