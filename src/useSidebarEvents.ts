import { useEffect } from "react";
import type { Feature, Point } from "geojson";
import type { GeoProperties } from "./mapData";

type UseSidebarEventsOptions = {
  isCollapsible: boolean;
  isBottomSheetMode: boolean;
  setIsListOpen: (value: boolean) => void;
  setSheetFeature: (value: Feature<Point, GeoProperties> | null) => void;
  select: (id: string | null) => void;
  spots: Feature<Point, GeoProperties>[];
};

function useSidebarEvents({
  isCollapsible,
  isBottomSheetMode,
  setIsListOpen,
  setSheetFeature,
  select,
  spots,
}: UseSidebarEventsOptions) {
  useEffect(() => {
    function closeSidebar() {
      if (!isCollapsible) return;
      setIsListOpen(false);
      select(null);
    }
    window.addEventListener("sidebar:request-close", closeSidebar);
    return () => {
      window.removeEventListener("sidebar:request-close", closeSidebar);
    };
  }, [isCollapsible, select, setIsListOpen]);

  useEffect(() => {
    function openSheet() {
      if (!isBottomSheetMode) return;
      setIsListOpen(false);
    }
    window.addEventListener("sidebar:request-open-sheet", openSheet);
    return () => {
      window.removeEventListener("sidebar:request-open-sheet", openSheet);
    };
  }, [isBottomSheetMode, setIsListOpen]);

  useEffect(() => {
    let clearSheetTimer: number | null = null;
    function handleLocationSelected(event: Event) {
      const detail = (event as CustomEvent<string | null>).detail;
      if (clearSheetTimer !== null) {
        window.clearTimeout(clearSheetTimer);
        clearSheetTimer = null;
      }

      if (!detail) {
        if (isCollapsible) {
          setIsListOpen(false);
        }
        clearSheetTimer = window.setTimeout(() => {
          setSheetFeature(null);
        }, 240);
        return;
      }

      const feature = spots.find((f) => f.id === detail) ?? null;
      if (feature) {
        setSheetFeature(feature);
      }

      if (!isCollapsible) return;
      if (isBottomSheetMode) {
        setIsListOpen(false);
        return;
      }
      setIsListOpen(true);
    }
    window.addEventListener("location:selected", handleLocationSelected);
    return () => {
      if (clearSheetTimer !== null) {
        window.clearTimeout(clearSheetTimer);
      }
      window.removeEventListener("location:selected", handleLocationSelected);
    };
  }, [isBottomSheetMode, isCollapsible, setIsListOpen, setSheetFeature, spots]);
}

export default useSidebarEvents;
