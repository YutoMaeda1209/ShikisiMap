import type { FeatureCollection, Point } from "geojson";
import { useState, type ReactNode } from "react";
import geoJsonRowData from "./data.json";
import { MapDataContext, type GeoProperties } from "./mapDataContext";

const spotsData = geoJsonRowData as FeatureCollection<Point, GeoProperties>;
const openSpotsData: FeatureCollection<Point, GeoProperties> = {
  ...spotsData,
  features: spotsData.features.filter((f) => !f.properties.isClosed),
};

function indexToId(index: number): string {
  const feature = spotsData.features[index];
  if (!feature) throw new Error(`No feature found at index ${index}`);
  return feature.id as string;
}

function idToIndex(id: string): number {
  const feature = spotsData.features.find((f) => f.id === id);
  if (!feature) throw new Error(`No feature found with id ${id}`);
  return spotsData.features.findIndex((f) => f.id === id);
}

function MapDataProvider({ children }: { children: ReactNode }) {
  const [query, setQuery] = useState<string | null>(null);
  const [mapDataRev, setMapDataRev] = useState<number>(0);

  const filterData = (newQuery: string) => {
    setQuery(newQuery);
    setMapDataRev((rev) => rev + 1);
  };

  const listSpotData: FeatureCollection<Point, GeoProperties> = {
    ...spotsData,
    features: query
      ? spotsData.features.filter((f) => {
          const p = f.properties;
          const q = query.trim().toLowerCase();
          return (
            p.name.toLowerCase().includes(q) ||
            p.address.toLowerCase().includes(q)
          );
        })
      : spotsData.features,
  };

  const mapSpotData: FeatureCollection<Point, GeoProperties> = {
    ...openSpotsData,
    features: query
      ? openSpotsData.features.filter((f) => {
          const p = f.properties;
          const q = query.trim().toLowerCase();
          return (
            p.name.toLowerCase().includes(q) ||
            p.address.toLowerCase().includes(q)
          );
        })
      : openSpotsData.features,
  };

  return (
    <MapDataContext.Provider
      value={{ listSpotData, mapSpotData, mapSpotDataRev: String(mapDataRev), indexToId, idToIndex, filterData }}
    >
      {children}
    </MapDataContext.Provider>
  );
}

export default MapDataProvider;
