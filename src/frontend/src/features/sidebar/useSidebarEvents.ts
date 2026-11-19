import { useEffect } from "react";
import type { Spot } from "../../lib/api";

type UseSidebarEventsOptions = {
  isCollapsible: boolean;
  isBottomSheetMode: boolean;
  setIsListOpen: (value: boolean) => void;
  setSheetSpot: (value: Spot | null) => void;
  select: (id: string | null) => void;
  spots: Spot[];
};

function useSidebarEvents({
  isCollapsible,
  isBottomSheetMode,
  setIsListOpen,
  setSheetSpot,
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
          setSheetSpot(null);
        }, 240);
        return;
      }

      const spot = spots.find((s) => s.id === detail) ?? null;
      if (spot) {
        setSheetSpot(spot);
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
  }, [isBottomSheetMode, isCollapsible, setIsListOpen, setSheetSpot, spots]);
}

export default useSidebarEvents;
