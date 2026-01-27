import { useEffect, useMemo, useRef, useState } from "react";
import logo from "./assets/img/logo.webp";
import { useLocationSelection } from "./locationSelectionContext";
import { spotsData } from "./mapData";
import BottomSheet from "./BottomSheet";
import "./Sidebar.css";
import SpotList from "./SpotList";
import useSidebarLayout from "./useSidebarLayout";

function Sidebar() {
  const [isSheetOpen, setIsSheetOpen] = useState(false);
  const [inlineVideoResetKey, setInlineVideoResetKey] = useState(0);
  const [sheetFeature, setSheetFeature] = useState<
    (typeof spotsData.features)[number] | null
  >(null);
  const [isListOpen, setIsListOpen] = useState(true);
  const prevCenteredRef = useRef(false);
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
        if (!nextState.isBottomSheetMode) {
          setIsSheetOpen(false);
        }
      }
    },
  });

  useEffect(() => {
    if (!isCollapsible) return;
    if (selectedId === null) return;
    if (isBottomSheetMode) return;
    setIsListOpen(true);
  }, [isBottomSheetMode, isCollapsible, selectedId]);

  useEffect(() => {
    if (!isBottomSheetMode) {
      setIsSheetOpen(false);
      setSheetFeature(null);
      return;
    }
    if (selectedId === null) {
      setIsSheetOpen(false);
      return;
    }
    setIsSheetOpen(true);
    setIsListOpen(false);
  }, [isBottomSheetMode, selectedId]);

  useEffect(() => {
    function handleRequestClose() {
      if (!isCollapsible) return;
      setIsListOpen(false);
      setIsSheetOpen(false);
      select(null);
    }
    window.addEventListener("sidebar:request-close", handleRequestClose);
    return () => {
      window.removeEventListener("sidebar:request-close", handleRequestClose);
    };
  }, [isCollapsible, select]);

  useEffect(() => {
    function handleRequestOpenSheet() {
      if (!isBottomSheetMode) return;
      if (selectedId === null) return;
      setIsSheetOpen(true);
      setIsListOpen(false);
    }
    window.addEventListener("sidebar:request-open-sheet", handleRequestOpenSheet);
    return () => {
      window.removeEventListener("sidebar:request-open-sheet", handleRequestOpenSheet);
    };
  }, [isBottomSheetMode, selectedId]);

  useEffect(() => {
    document.body.classList.toggle("sidebar-centered", isCentered);
    return () => {
      document.body.classList.remove("sidebar-centered");
    };
  }, [isCentered]);

  useEffect(() => {
    if (!prevCenteredRef.current && isCentered) {
      setInlineVideoResetKey((prev) => prev + 1);
    }
    prevCenteredRef.current = isCentered;
  }, [isCentered]);

  const selectedFeature = useMemo(() => {
    if (!selectedId) return null;
    return spotsData.features.find((f) => f.id === selectedId) ?? null;
  }, [selectedId]);

  useEffect(() => {
    if (selectedFeature) {
      setSheetFeature(selectedFeature);
      return;
    }
    if (!isSheetOpen) {
      const t = window.setTimeout(() => {
        setSheetFeature(null);
      }, 240);
      return () => window.clearTimeout(t);
    }
  }, [isSheetOpen, selectedFeature]);

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
          setIsSheetOpen(false);
        }}
        onRequestClose={() => setIsListOpen(false)}
      />
      {isBottomSheetMode && sheetFeature ? (
        <BottomSheet
          feature={sheetFeature}
          isOpen={isSheetOpen}
          onRequestClose={() => {
            setIsSheetOpen(false);
            select(null);
          }}
        />
      ) : null}
    </div>
  );
}

export default Sidebar;
