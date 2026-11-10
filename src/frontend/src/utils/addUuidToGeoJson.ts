import fs from "fs";
import type { FeatureCollection, Geometry } from "geojson";
import path from "path";
import { v4 as uuidv4 } from "uuid";
import type { GeoProperties } from "../mapDataContext";

const inputFilePath = process.argv[2]
if (!inputFilePath) {
    console.error("Please provide the path to the GeoJSON file.");
    process.exit(1);
}

const filePath = path.isAbsolute(inputFilePath)
    ? inputFilePath
    : path.resolve(process.cwd(), inputFilePath);

const rawJson = fs.readFileSync(filePath, "utf-8");
const geoJsonRowData = JSON.parse(rawJson) as FeatureCollection<Geometry, GeoProperties>;

const spotsData = geoJsonRowData as FeatureCollection<Geometry, GeoProperties>;

spotsData.features.forEach((feature) => {
    if (feature.id !== undefined) return;
    feature.id = uuidv4();
});

fs.writeFileSync(filePath, JSON.stringify(spotsData, null, 2), "utf-8");
