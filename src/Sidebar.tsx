import { useCallback, useEffect, useRef, useState } from "react";
import logo from "./assets/img/logo.webp";
import { useLocationSelection } from "./locationSelectionContext";
import { spotsData } from "./mapData";
import BottomSheet from "./BottomSheet";
import "./Sidebar.css";
import SpotList from "./SpotList";
import useSidebarLayout from "./useSidebarLayout";

function Sidebar() {
  const [inlineVideoResetKey, setInlineVideoResetKey] = useState(0);
  const [sheetFeature, setSheetFeature] = useState<
    (typeof spotsData.features)[number] | null
  >(null);
  const [isListOpen, setIsListOpen] = useState(true);
  const clearSheetTimerRef = useRef<number | null>(null);
  const { selectedId, select } = useLocationSelection();
  const {
    sidebarRef,
    titleRef,
    listHeight,
    isCollapsible,
    isCentered,
    isBottomSheetMode,
  } = useSidebarLayout({
    isListOpen,
    centeredThreshold: 0.4,
    collapsibleThreshold: 0.4,
    onLayoutChange: (prevState, nextState) => {
      if (prevState.isCollapsible !== nextState.isCollapsible) {
        setIsListOpen(!nextState.isCollapsible);
      }
      if (!prevState.isCentered && nextState.isCentered) {
        setInlineVideoResetKey((prev) => prev + 1);
      }
    },
  });

  const isSheetOpen = isBottomSheetMode && selectedId !== null && !isListOpen;

  const closeSidebar = useCallback(() => {
    if (!isCollapsible) return;
    setIsListOpen(false);
    select(null);
  }, [isCollapsible, select]);

  const openSheet = useCallback(() => {
    if (!isBottomSheetMode) return;
    if (selectedId === null) return;
    setIsListOpen(false);
  }, [isBottomSheetMode, selectedId]);

  const handleLocationSelected = useCallback(
    (event: Event) => {
      const detail = (event as CustomEvent<string | null>).detail;
      if (clearSheetTimerRef.current !== null) {
        window.clearTimeout(clearSheetTimerRef.current);
        clearSheetTimerRef.current = null;
      }

      if (!detail) {
        if (isCollapsible) {
          setIsListOpen(false);
        }
        clearSheetTimerRef.current = window.setTimeout(() => {
          setSheetFeature(null);
        }, 240);
        return;
      }

      const feature =
        spotsData.features.find((f) => f.id === detail) ?? null;
      if (feature) {
        setSheetFeature(feature);
      }

      if (!isCollapsible) return;
      if (isBottomSheetMode) {
        setIsListOpen(false);
        return;
      }
      setIsListOpen(true);
    },
    [isBottomSheetMode, isCollapsible],
  );

  useEffect(() => {
    window.addEventListener("sidebar:request-close", closeSidebar);
    return () => {
      window.removeEventListener("sidebar:request-close", closeSidebar);
    };
  }, [closeSidebar]);

  useEffect(() => {
    window.addEventListener("sidebar:request-open-sheet", openSheet);
    return () => {
      window.removeEventListener("sidebar:request-open-sheet", openSheet);
    };
  }, [openSheet]);

  useEffect(() => {
    document.body.classList.toggle("sidebar-centered", isCentered);
    return () => {
      document.body.classList.remove("sidebar-centered");
    };
  }, [isCentered]);

  useEffect(() => {
    window.addEventListener("location:selected", handleLocationSelected);
    return () => {
      window.removeEventListener("location:selected", handleLocationSelected);
    };
  }, [handleLocationSelected]);

  const sidebarClassName = [
    isCollapsible && !isListOpen ? "collapsed" : "",
    isCentered ? "centered" : "",
  ]
    .filter(Boolean)
    .join(" ");

  return (
    <div
      id="sidebarComponent"
      ref={sidebarRef}
      className={sidebarClassName}
      onClick={(event) => event.stopPropagation()}
    >
      <div id="title" ref={titleRef}>
        <img id="logo" src={logo} alt="Logo" />
        <span id="subtitle">非公式 敷嶋てとら 聖地巡礼マップ</span>
      </div>
      <SpotList
        height={listHeight}
        isCollapsible={isCollapsible}
        isOpen={!isCollapsible || isListOpen}
        disableInlineVideo={isCentered}
        inlineVideoResetKey={inlineVideoResetKey}
        onRequestOpen={() => {
          setIsListOpen(true);
        }}
        onRequestClose={() => setIsListOpen(false)}
      />
      {isBottomSheetMode && sheetFeature ? (
        <BottomSheet
          feature={sheetFeature}
          isOpen={isSheetOpen}
          onRequestClose={closeSidebar}
        />
      ) : null}
    </div>
  );
}

export default Sidebar;
