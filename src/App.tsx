import { useSyncExternalStore } from "react";
import LocationSelectionProvider from "./LocationSelectionProvider";
import Map from "./Map";
import MapDataProvider from "./MapDataProvider";
import Sidebar from "./Sidebar";
import MobileNavigation from "./mobile/MobileNavigation";
import UrlManager from "./UrlManager";

const MOBILE_QUERY = "(max-width: 767px)";

function subscribeToMobileQuery(onStoreChange: () => void) {
  const mediaQuery = window.matchMedia(MOBILE_QUERY);
  mediaQuery.addEventListener("change", onStoreChange);
  return () => mediaQuery.removeEventListener("change", onStoreChange);
}

function getMobileSnapshot() {
  return window.matchMedia(MOBILE_QUERY).matches;
}

function App() {
  const isMobile = useSyncExternalStore(
      subscribeToMobileQuery,
      getMobileSnapshot,
      () => false,
    );

  return (
    <div className="relative h-full min-h-dvh w-full md:flex">
      <LocationSelectionProvider>
        <MapDataProvider>
          {isMobile ? <MobileNavigation /> : <Sidebar />}
          <Map />
          <UrlManager />
        </MapDataProvider>
      </LocationSelectionProvider>
    </div>
  );
}

export default App;
