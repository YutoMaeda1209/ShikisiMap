import type { FeatureCollection, Point } from "geojson";
import geoJsonRowData from "./data.json";

export type GeoProperties = {
    name: string;
    address: string;
    isClosed: boolean;
    youtubeId: string;
    timestamp: number;
};

const spotsData = geoJsonRowData as FeatureCollection<Point, GeoProperties>;

const openSpotsData: FeatureCollection<Point, GeoProperties> = {
    ...spotsData,
    features: spotsData.features.filter((f) => !f.properties.isClosed),
};

export { openSpotsData, spotsData };

export function indexToId(index: number): string {
    const feature = spotsData.features[index];
    if (!feature) throw new Error(`No feature found at index ${index}`);
    return feature.id as string;
}

export function idToIndex(id: string): number {
    const feature = spotsData.features.find((f) => f.id === id);
    if (!feature) throw new Error(`No feature found with id ${id}`);
    return spotsData.features.findIndex((f) => f.id === id);
}
