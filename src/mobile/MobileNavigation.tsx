import type { Feature, Point } from "geojson";
import { useEffect, useState } from "react";
import logo from "../assets/img/logo.webp";
import { useLocationSelection } from "../locationSelectionContext";
import { useMapData, type GeoProperties } from "../mapDataContext";
import SpotList from "../SpotList";
import BottomSheet from "./BottomSheet";
import MobileSpotItem from "./MobileSpotItem";

function getListHeight() {
  return Math.max(240, Math.min(window.innerHeight - 150, 640));
}

function MobileNavigation() {
  const { allSpotData } = useMapData();
  const { selectedId, select } = useLocationSelection();
  const initialFeature =
    selectedId === null
      ? null
      : (allSpotData.features.find((feature) => feature.id === selectedId) ??
        null);
  const [isListOpen, setIsListOpen] = useState(selectedId === null);
  const [listHeight, setListHeight] = useState(getListHeight);
  const [sheetFeature, setSheetFeature] = useState<
    Feature<Point, GeoProperties> | null
  >(initialFeature);

  useEffect(() => {
    const updateHeight = () => setListHeight(getListHeight());
    window.addEventListener("resize", updateHeight);
    window.addEventListener("orientationchange", updateHeight);
    return () => {
      window.removeEventListener("resize", updateHeight);
      window.removeEventListener("orientationchange", updateHeight);
    };
  }, []);

  useEffect(() => {
    let clearTimer: number | null = null;

    const handleSelection = (event: Event) => {
      const id = (event as CustomEvent<string | null>).detail;
      if (id === null) {
        clearTimer = window.setTimeout(() => setSheetFeature(null), 240);
        return;
      }
      const feature =
        allSpotData.features.find((candidate) => candidate.id === id) ?? null;
      setSheetFeature(feature);
      setIsListOpen(false);
    };

    const closeNavigation = () => {
      setIsListOpen(false);
      if (selectedId !== null) select(null);
    };

    const openSheet = () => setIsListOpen(false);

    window.addEventListener("location:selected", handleSelection);
    window.addEventListener("sidebar:request-close", closeNavigation);
    window.addEventListener("sidebar:request-open-sheet", openSheet);
    return () => {
      if (clearTimer !== null) window.clearTimeout(clearTimer);
      window.removeEventListener("location:selected", handleSelection);
      window.removeEventListener("sidebar:request-close", closeNavigation);
      window.removeEventListener("sidebar:request-open-sheet", openSheet);
    };
  }, [allSpotData, select, selectedId]);

  return (
    <nav className="pointer-events-none fixed inset-0 z-500 w-full">
      {isListOpen ? (
        <div className="pointer-events-auto absolute top-2 right-2 left-2 max-h-[calc(100dvh-16px)] overflow-hidden rounded-2xl bg-white/90 p-2 shadow-lg backdrop-blur-sm">
          <img className="mx-auto w-36" src={logo} alt="Logo" />
          <SpotList
            height={listHeight}
            isCollapsible={true}
            isOpen={true}
            disableInlineVideo={false}
            inlineVideoResetKey={0}
            onRequestOpen={() => setIsListOpen(true)}
            onRequestClose={() => setIsListOpen(false)}
            rowComponent={MobileSpotItem}
          />
        </div>
      ) : (
        <button
          type="button"
          className="pointer-events-auto absolute top-3 left-3 rounded-full bg-white/90 px-4 py-2 font-bold shadow-lg backdrop-blur-sm"
          onClick={() => setIsListOpen(true)}
        >
          スポット一覧
        </button>
      )}
      {sheetFeature ? (
        <BottomSheet
          feature={sheetFeature}
          isOpen={selectedId !== null && !isListOpen}
          onRequestClose={() => select(null)}
        />
      ) : null}
    </nav>
  );
}

export default MobileNavigation;
