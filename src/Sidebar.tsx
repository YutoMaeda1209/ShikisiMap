import { useEffect, useState } from "react";
import logo from "./assets/img/logo.webp";
import BottomSheet from "./BottomSheet";
import { useLocationSelection } from "./locationSelectionContext";
import { spotsData } from "./mapData";
import "./Sidebar.css";
import SpotList from "./SpotList";
import useSidebarEvents from "./useSidebarEvents";
import useSidebarLayout from "./useSidebarLayout";

function Sidebar() {
  const [inlineVideoResetKey, setInlineVideoResetKey] = useState(0);
  const [sheetFeature, setSheetFeature] = useState<
    (typeof spotsData.features)[number] | null
  >(null);
  const [isListOpen, setIsListOpen] = useState(true);
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

  useSidebarEvents({
    isCollapsible,
    isBottomSheetMode,
    setIsListOpen,
    setSheetFeature,
    select,
    spots: spotsData.features,
  });

  useEffect(() => {
    document.body.classList.toggle("sidebar-centered", isCentered);
    return () => {
      document.body.classList.remove("sidebar-centered");
    };
  }, [isCentered]);

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
        <span id="subtitle">
          非公式 敷嶋てとら <wbr />
          聖地巡礼マップ
        </span>
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
          onRequestClose={() => {
            if (!isCollapsible) return;
            setIsListOpen(false);
            select(null);
          }}
        />
      ) : null}
    </div>
  );
}

export default Sidebar;
