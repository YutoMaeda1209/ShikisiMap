import { createContext, useContext } from "react";
import type { Spot } from "./api";

export type VisitFilter = "all" | "visited" | "unvisited";

export type MapDataContextType = {
  /** Every spot of this page, whatever the search and the filter. */
  allSpots: Spot[];
  /** The spots matching the search and the filter, for the list and the map. */
  shownSpots: Spot[];
  status: "loading" | "ready" | "error";
  reload: () => void;
  setQuery: (query: string) => void;
  visitFilter: VisitFilter;
  setVisitFilter: (filter: VisitFilter) => void;
};

export const MapDataContext = createContext<MapDataContextType | undefined>(undefined);

export function useMapData(): MapDataContextType {
  const ctx = useContext(MapDataContext);
  if (!ctx) throw new Error('useMapData must be used within a MapDataProvider');
  return ctx;
}
