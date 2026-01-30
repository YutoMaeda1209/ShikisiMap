import type { FeatureCollection, Point } from "geojson";
import { createContext, useContext } from "react";

export type GeoProperties = {
    name: string;
    address: string;
    isClosed: boolean;
    youtubeId: string;
    timestamp: number;
};

type MapDataContextType = {
    listSpotData: FeatureCollection<Point, GeoProperties>;
    mapSpotData: FeatureCollection<Point, GeoProperties>;
    mapSpotDataRev: string;
    indexToId: (index: number) => string;
    idToIndex: (id: string) => number;
    filterData: (query: string) => void;
};

export const MapDataContext = createContext<MapDataContextType | undefined>(undefined);

export function useMapData(): MapDataContextType {
    const ctx = useContext(MapDataContext);
    if (!ctx) throw new Error('useMapData must be used within a MapDataProvider');
    return ctx;
}
