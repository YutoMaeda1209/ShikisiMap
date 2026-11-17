import AuthProvider from "../features/auth/AuthProvider";
import DialogHost from "../features/dialogs/DialogHost";
import MapView from "../features/map/MapView";
import Sidebar from "../features/sidebar/Sidebar";
import { VisitorStatus } from "../features/sidebar/SidebarStatus";
import LocationSelectionProvider from "../features/spots/LocationSelectionProvider";
import MapDataProvider from "../features/spots/MapDataProvider";
import UrlManager from "../features/spots/UrlManager";
import VisitsProvider from "../features/visits/VisitsProvider";

// The map with the visitor's own visit records.
function MapPage() {
  return (
    <AuthProvider>
      <LocationSelectionProvider>
        <VisitsProvider>
          <MapDataProvider>
            <div id="sidebarContainer">
              <Sidebar status={<VisitorStatus />} />
            </div>
            <MapView />
            <DialogHost />
            <UrlManager />
          </MapDataProvider>
        </VisitsProvider>
      </LocationSelectionProvider>
    </AuthProvider>
  );
}

export default MapPage;
