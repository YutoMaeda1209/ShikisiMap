import * as maplibregl from "maplibre-gl";
import type { Map as MapLibreMap, Marker } from "maplibre-gl";
import "maplibre-gl/dist/maplibre-gl.css";
import { useEffect, useRef, useState } from "react";
import CurrentLocationMarker from "./CurrentLocationMarker";
import { useLocationSelection } from "./locationSelectionContext";
import { useMapData } from "./mapDataContext";

const INITIAL_CENTER: [number, number] = [139.650027, 35.676423];
const SIDEBAR_FOCUS_ZOOM = 16;

type FocusSpotEventDetail = {
  id: string;
  coordinates: [number, number];
};

const mapStyle: maplibregl.StyleSpecification = {
  version: 8,
  sources: {
    openStreetMap: {
      type: "raster",
      tiles: [
        "https://a.tile.openstreetmap.org/{z}/{x}/{y}.png",
        "https://b.tile.openstreetmap.org/{z}/{x}/{y}.png",
        "https://c.tile.openstreetmap.org/{z}/{x}/{y}.png",
      ],
      tileSize: 256,
      attribution:
        '&copy; <a href="https://www.openstreetmap.org/copyright">OpenStreetMap</a> contributors',
    },
  },
  layers: [{ id: "openStreetMap", type: "raster", source: "openStreetMap" }],
};

function createSpotMarkerElement(
  name: string,
  isSelected: boolean,
  onSelect: () => void,
) {
  const button = document.createElement("button");
  button.type = "button";
  button.dataset.spotMarker = "true";
  button.className = isSelected
    ? "relative h-[47px] w-[29px] cursor-pointer border-0 bg-transparent p-0 focus-visible:outline-3 focus-visible:outline-offset-2 focus-visible:outline-black"
    : "relative h-[41px] w-[25px] cursor-pointer border-0 bg-transparent p-0 focus-visible:outline-3 focus-visible:outline-offset-2 focus-visible:outline-black";
  button.setAttribute("aria-label", name);
  button.title = name;

  const pin = document.createElement("span");
  pin.className = isSelected
    ? "absolute top-0.5 left-0.5 box-border h-[25px] w-[25px] -rotate-45 rounded-[50%_50%_50%_0] border-2 border-white bg-[#d95f18] shadow-[1px_1px_3px_rgb(0_0_0/45%)] after:absolute after:top-1.5 after:left-1.5 after:h-[9px] after:w-[9px] after:rounded-full after:bg-white after:content-['']"
    : "absolute top-0.5 left-0.5 box-border h-[21px] w-[21px] -rotate-45 rounded-[50%_50%_50%_0] border-2 border-white bg-[#2878c8] shadow-[1px_1px_3px_rgb(0_0_0/45%)] after:absolute after:top-[5px] after:left-[5px] after:h-[7px] after:w-[7px] after:rounded-full after:bg-white after:content-['']";
  button.append(pin);

  button.addEventListener("click", (event) => {
    event.stopPropagation();
    onSelect();
  });
  return button;
}

function Map() {
  const { selectedId, select } = useLocationSelection();
  const { listSpotData, mapSpotData } = useMapData();
  const containerRef = useRef<HTMLDivElement>(null);
  const mapRef = useRef<MapLibreMap | null>(null);
  const markersRef = useRef<Marker[]>([]);
  const selectRef = useRef(select);
  const focusedFromSidebarRef = useRef<string | null>(null);
  const [map, setMap] = useState<MapLibreMap | null>(null);

  useEffect(() => {
    selectRef.current = select;
  }, [select]);

  useEffect(() => {
    if (!containerRef.current) return;
    const nextMap = new maplibregl.Map({
      container: containerRef.current,
      style: mapStyle,
      center: INITIAL_CENTER,
      zoom: 14,
      attributionControl: { compact: true },
    });
    const closeSidebar = (event: maplibregl.MapMouseEvent) => {
      const target = event.originalEvent.target;
      if (
        target instanceof Element &&
        target.closest('[data-spot-marker="true"]')
      ) {
        return;
      }
      window.dispatchEvent(new CustomEvent("sidebar:request-close"));
    };
    const focusSpot = (event: Event) => {
      const { id, coordinates } = (
        event as CustomEvent<FocusSpotEventDetail>
      ).detail;
      focusedFromSidebarRef.current = id;
      nextMap.easeTo({
        center: coordinates,
        zoom: Math.max(nextMap.getZoom(), SIDEBAR_FOCUS_ZOOM),
        duration: 1200,
      });
    };
    nextMap.on("click", closeSidebar);
    window.addEventListener("map:focus-spot", focusSpot);
    mapRef.current = nextMap;
    setMap(nextMap);
    return () => {
      nextMap.off("click", closeSidebar);
      window.removeEventListener("map:focus-spot", focusSpot);
      nextMap.remove();
      mapRef.current = null;
      setMap(null);
    };
  }, []);

  useEffect(() => {
    if (!mapRef.current || selectedId === null) return;
    const feature = listSpotData.features.find(
      (candidate) => candidate.id === selectedId,
    );
    if (feature) {
      if (focusedFromSidebarRef.current === selectedId) {
        focusedFromSidebarRef.current = null;
      } else {
        focusedFromSidebarRef.current = null;
        mapRef.current.panTo(feature.geometry.coordinates as [number, number]);
      }
    }
  }, [selectedId, listSpotData]);

  useEffect(() => {
    if (!map) return;
    markersRef.current.forEach((marker) => marker.remove());
    markersRef.current = mapSpotData.features.map((feature) => {
      const element = createSpotMarkerElement(
        feature.properties.name,
        feature.id === selectedId,
        () => {
          const id = feature.id as string;
          window.dispatchEvent(
            new CustomEvent("map:focus-spot", {
              detail: {
                id,
                coordinates: feature.geometry.coordinates,
              },
            }),
          );
          selectRef.current(id);
        },
      );
      return new maplibregl.Marker({ element, anchor: "bottom" })
        .setLngLat(feature.geometry.coordinates as [number, number])
        .addTo(map);
    });
    return () => {
      markersRef.current.forEach((marker) => marker.remove());
      markersRef.current = [];
    };
  }, [map, mapSpotData, selectedId]);

  return (
    <>
      <div ref={containerRef} className="absolute inset-0 h-screen w-full" />
      <CurrentLocationMarker map={map} />
    </>
  );
}

export default Map;
