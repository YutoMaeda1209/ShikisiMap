import type { MutableRefObject } from "react";
import {
  useCallback,
  useEffect,
  useLayoutEffect,
  useRef,
  useState,
} from "react";

type LayoutState = {
  isCollapsible: boolean;
  isCentered: boolean;
  isBottomSheetMode: boolean;
};

type UseSidebarLayoutOptions = {
  isListOpen: boolean;
  centeredThreshold?: number;
  collapsibleThreshold?: number;
  onLayoutChange?: (prev: LayoutState, next: LayoutState) => void;
};

type UseSidebarLayoutResult = LayoutState & {
  sidebarRef: MutableRefObject<HTMLDivElement | null>;
  titleRef: MutableRefObject<HTMLDivElement | null>;
  listHeight: number;
};

function useSidebarLayout({
  isListOpen,
  centeredThreshold = 0.4,
  collapsibleThreshold = 0.4,
  onLayoutChange,
}: UseSidebarLayoutOptions): UseSidebarLayoutResult {
  const sidebarRef = useRef<HTMLDivElement | null>(null);
  const titleRef = useRef<HTMLDivElement | null>(null);
  const [listHeight, setListHeight] = useState<number>(0);
  const [isCollapsible, setIsCollapsible] = useState(false);
  const [isCentered, setIsCentered] = useState(false);
  const [isBottomSheetMode, setIsBottomSheetMode] = useState(false);
  const prevLayoutRef = useRef<LayoutState>({
    isCollapsible: false,
    isCentered: false,
    isBottomSheetMode: false,
  });

  const updateListHeight = useCallback(() => {
    if (!sidebarRef.current || !titleRef.current) return;
    const sidebarH = sidebarRef.current.clientHeight;
    const titleH = titleRef.current.clientHeight;
    const extraOffset = isCollapsible && isListOpen ? 40 : 0;
    const offset = 50 + extraOffset;
    const newHeight = Math.max(0, Math.floor(sidebarH - titleH - offset));
    setListHeight(newHeight);
  }, [isCollapsible, isListOpen]);

  useLayoutEffect(() => {
    updateListHeight();
    const roSidebar = new ResizeObserver(updateListHeight);
    const roTitle = new ResizeObserver(updateListHeight);
    if (sidebarRef.current) roSidebar.observe(sidebarRef.current);
    if (titleRef.current) roTitle.observe(titleRef.current);

    return () => {
      roSidebar.disconnect();
      roTitle.disconnect();
    };
  }, [updateListHeight]);

  useEffect(() => {
    if (!isCollapsible || !isListOpen) return;
    updateListHeight();
    const t = window.setTimeout(updateListHeight, 220);
    return () => window.clearTimeout(t);
  }, [isCollapsible, isListOpen, updateListHeight]);

  useLayoutEffect(() => {
    function updateLayout() {
      if (!sidebarRef.current) return;
      const sidebarW = sidebarRef.current.clientWidth;
      const viewportW = window.innerWidth;
      const nextIsCollapsible = sidebarW >= viewportW * collapsibleThreshold;
      const nextIsCentered = sidebarW >= viewportW * centeredThreshold;
      const nextIsBottomSheetMode = sidebarW >= viewportW * centeredThreshold;
      const nextState: LayoutState = {
        isCollapsible: nextIsCollapsible,
        isCentered: nextIsCentered,
        isBottomSheetMode: nextIsBottomSheetMode,
      };

      setIsCollapsible(nextIsCollapsible);
      setIsCentered(nextIsCentered);
      setIsBottomSheetMode(nextIsBottomSheetMode);

      const prevState = prevLayoutRef.current;
      if (
        prevState.isCollapsible !== nextState.isCollapsible ||
        prevState.isCentered !== nextState.isCentered ||
        prevState.isBottomSheetMode !== nextState.isBottomSheetMode
      ) {
        onLayoutChange?.(prevState, nextState);
        prevLayoutRef.current = nextState;
      }
    }

    updateLayout();
    const roSidebar = new ResizeObserver(updateLayout);
    if (sidebarRef.current) roSidebar.observe(sidebarRef.current);
    window.addEventListener("resize", updateLayout);
    return () => {
      roSidebar.disconnect();
      window.removeEventListener("resize", updateLayout);
    };
  }, [centeredThreshold, collapsibleThreshold, onLayoutChange]);

  return {
    sidebarRef,
    titleRef,
    listHeight,
    isCollapsible,
    isCentered,
    isBottomSheetMode,
  };
}

export default useSidebarLayout;
