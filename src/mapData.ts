import type { FeatureCollection, Point } from "geojson";
import geoJsonRowData from "./data.json";

export type GeoProperties = {
    name: string;
    category: string;
    address: string;
    isClosed: boolean;
    youtubeId: string;
    timestamp: number;
};

const spotsData = geoJsonRowData as FeatureCollection<Point, GeoProperties>;

spotsData.features.forEach((feature, idx) => {
    feature.id = idx;
});

const openSpotsData: FeatureCollection<Point, GeoProperties> = {
    ...spotsData,
    features: spotsData.features.filter((f) => !f.properties.isClosed),
};

export { openSpotsData, spotsData };
