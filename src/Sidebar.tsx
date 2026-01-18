import { useLayoutEffect, useRef, useState } from "react";
import logo from "./assets/img/logo.webp";
import "./Sidebar.css";
import SpotList from "./SpotList";

function Sidebar() {
  const sidebarRef = useRef<HTMLDivElement | null>(null);
  const titleRef = useRef<HTMLDivElement | null>(null);
  const [listHeight, setListHeight] = useState<number>(0);

  useLayoutEffect(() => {
    function updateHeight() {
      if (!sidebarRef.current || !titleRef.current) return;
      const sidebarH = sidebarRef.current.clientHeight;
      const titleH = titleRef.current.clientHeight;
      const offset = 50;
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
  }, []);

  return (
    <div id="sidebarComponent" ref={sidebarRef}>
      <div id="title" ref={titleRef}>
        <img id="logo" src={logo} alt="Logo" />
        <span id="subtitle">敷嶋てとら ファンメイド<br />聖地巡礼マップ</span>
      </div>
      <SpotList height={listHeight} />
    </div>
  );
}

export default Sidebar;
