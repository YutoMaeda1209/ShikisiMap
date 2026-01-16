import type { LatLngExpression } from 'leaflet';
import { createContext, useContext } from 'react';

export type MapControls = {
  panTo: (latlng: LatLngExpression, zoom?: number) => void;
  flyTo: (latlng: LatLngExpression, zoom?: number) => void;
};

export const MapContext = createContext<MapControls | undefined>(undefined);

export function useMapControls(): MapControls {
  const ctx = useContext(MapContext);
  if (!ctx) throw new Error('useMapControls must be used within MapContext.Provider');
  return ctx;
};
