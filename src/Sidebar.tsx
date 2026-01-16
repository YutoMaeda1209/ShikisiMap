import type { Feature, FeatureCollection, Geometry } from "geojson";
import { useEffect, useRef } from "react";
import ListItem from "./ListItem";
import "./Sidebar.css";
import logo from "./assets/img/logo.webp";
import geoJsonRowData from "./data.json";
import type { GeoProperties } from "./types";

function Sidebar() {
  const titleRef = useRef<HTMLDivElement>(null);
  const spotListRef = useRef<HTMLUListElement>(null);

  // Prepare list items from GeoJSON data
  const features = (geoJsonRowData as FeatureCollection<Geometry, GeoProperties>).features;
  const listItems = features.map((f: Feature<Geometry, GeoProperties>, i: number) => (
    <ListItem feature={f} index={i} key={i} />
  ));

  // Sync padding-top of items to height of title
  useEffect(() => {
    function onTitleResize() {
      if (titleRef.current && spotListRef.current) {
        const h = titleRef.current.offsetHeight;
        spotListRef.current.style.paddingTop = `${h}px`;
        spotListRef.current.style.scrollPaddingTop = `${h}px`;
      }
    }
    onTitleResize();
    const ro = new ResizeObserver(onTitleResize);
    if (titleRef.current) ro.observe(titleRef.current);
    return () => {
      ro.disconnect();
    };
  }, []);

  return (
    <div id="sidebarComponent">
      <div id="listContent">
        <div id="title" ref={titleRef}>
          <img id="logo" src={logo} alt="Logo" />
          <span id="subtitle">敷嶋てとら ファンメイド<br />聖地巡礼マップ</span>
        </div>
        <ul id="spotList" ref={spotListRef}>
          {features.length !== 0 ? listItems : <li>データが見つかりません</li>}
        </ul>
      </div>
    </div>
  );
}

export default Sidebar;
