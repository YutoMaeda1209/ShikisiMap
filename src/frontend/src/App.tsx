import "./App.css";
import GitHubRibbon from "./GitHubRibbon";
import LocationSelectionProvider from "./LocationSelectionProvider";
import Map from "./Map";
import MapDataProvider from "./MapDataProvider";
import Sidebar from "./Sidebar";
import UrlManager from "./UrlManager";

function App() {
  return (
    <div id="app">
      <LocationSelectionProvider>
        <MapDataProvider>
          <div id="sidebarContainer">
            <Sidebar />
          </div>
          <Map />
          <GitHubRibbon />
          <UrlManager />
        </MapDataProvider>
      </LocationSelectionProvider>
    </div>
  );
}

export default App;
