import { useEffect, useLayoutEffect, useRef, useState } from "react";
import logo from "./assets/img/logo.webp";
import { useLocationSelection } from "./locationSelectionContext";
import "./Sidebar.css";
import SpotList from "./SpotList";

function Sidebar() {
  const sidebarRef = useRef<HTMLDivElement | null>(null);
  const titleRef = useRef<HTMLDivElement | null>(null);
  const [listHeight, setListHeight] = useState<number>(0);
  const [isCollapsible, setIsCollapsible] = useState(false);
  const [isCentered, setIsCentered] = useState(false);
  const [isListOpen, setIsListOpen] = useState(true);
  const { selectedId } = useLocationSelection();

  useLayoutEffect(() => {
    function updateHeight() {
      if (!sidebarRef.current || !titleRef.current) return;
      const sidebarH = sidebarRef.current.clientHeight;
      const titleH = titleRef.current.clientHeight;
      const extraOffset = isCollapsible && isListOpen ? 40 : 0;
      const offset = 50 + extraOffset;
      const newHeight = Math.max(0, Math.floor(sidebarH - titleH - offset));
      setListHeight(newHeight);
    }
    updateHeight();

    const roSidebar = new ResizeObserver(updateHeight);
    const roTitle = new ResizeObserver(updateHeight);
    if (sidebarRef.current) roSidebar.observe(sidebarRef.current);
    if (titleRef.current) roTitle.observe(titleRef.current);

    return () => {
      roSidebar.disconnect();
      roTitle.disconnect();
    };
  }, [isCollapsible, isListOpen]);

  useEffect(() => {
    if (!isCollapsible || !isListOpen) return;
    function updateOpenHeight() {
      if (!sidebarRef.current || !titleRef.current) return;
      const sidebarH = sidebarRef.current.clientHeight;
      const titleH = titleRef.current.clientHeight;
      const extraOffset = isCollapsible && isListOpen ? 40 : 0;
      const offset = 50 + extraOffset;
      const newHeight = Math.max(0, Math.floor(sidebarH - titleH - offset));
      setListHeight(newHeight);
    }
    updateOpenHeight();
    window.setTimeout(updateOpenHeight, 220);
  }, [isCollapsible, isListOpen]);

  useLayoutEffect(() => {
    function updateCollapsible() {
      if (!sidebarRef.current) return;
      const sidebarW = sidebarRef.current.clientWidth;
      const viewportW = window.innerWidth;
      setIsCollapsible(sidebarW >= viewportW * 0.4);
      setIsCentered(sidebarW >= viewportW * 0.5);
    }
    updateCollapsible();
    const roSidebar = new ResizeObserver(updateCollapsible);
    if (sidebarRef.current) roSidebar.observe(sidebarRef.current);
    window.addEventListener("resize", updateCollapsible);
    return () => {
      roSidebar.disconnect();
      window.removeEventListener("resize", updateCollapsible);
    };
  }, []);

  useEffect(() => {
    if (!isCollapsible) {
      setIsListOpen(true);
      return;
    }
    setIsListOpen(false);
  }, [isCollapsible]);

  useEffect(() => {
    if (!isCollapsible) return;
    if (selectedId === null) return;
    setIsListOpen(true);
  }, [isCollapsible, selectedId]);

  useEffect(() => {
    function handleRequestClose() {
      if (!isCollapsible) return;
      setIsListOpen(false);
    }
    window.addEventListener("sidebar:request-close", handleRequestClose);
    return () => {
      window.removeEventListener("sidebar:request-close", handleRequestClose);
    };
  }, [isCollapsible]);

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
        onRequestOpen={() => setIsListOpen(true)}
        onRequestClose={() => setIsListOpen(false)}
      />
    </div>
  );
}

export default Sidebar;
