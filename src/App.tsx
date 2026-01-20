import type { LatLngExpression, Map as LeafletMap } from 'leaflet';
import { useState } from 'react';
import './App.css';
import Map from './Map';
import type { MapControls } from './MapContext';
import { MapContext } from './MapContext';
import Sidebar from './Sidebar';

export const rowHeight = 300;

function App() {
  const [map, setMap] = useState<LeafletMap | null>(null);

  const controls: MapControls = {
    panTo: (latlng: LatLngExpression, zoom?: number) => {
      if (!map) return;
      map.setView(latlng, zoom);
    },
    flyTo: (latlng: LatLngExpression, zoom?: number) => {
      if (!map) return;
      map.flyTo(latlng, zoom);
    }
  };

  return (
    <div id="app">
      <MapContext.Provider value={controls}>
        <div id="sidebarContainer">
          <Sidebar />
        </div>
        <Map onMapCreated={setMap} />
      </MapContext.Provider>
    </div>
  );
}

export default App;
