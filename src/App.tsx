import './App.css';
import GitHubRibbon from './GitHubRibbon';
import LocationSelectionProvider from './LocationSelectionProvider';
import Map from './Map';
import Sidebar from './Sidebar';
import UrlManager from './UrlManager';

function App() {
  return (
    <div id="app">
      <LocationSelectionProvider>
        <div id="sidebarContainer">
          <Sidebar />
        </div>
        <Map />
        <GitHubRibbon />
        <UrlManager />
      </LocationSelectionProvider>
    </div>
  );
}

export default App;
