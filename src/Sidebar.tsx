import type { Feature, FeatureCollection, Geometry } from "geojson";
import { useEffect, useRef } from "react";
import "./Sidebar.css";
import logo from "./assets/img/logo.webp";
import geoJsonRowData from "./data.json";
import type { GeoProperties } from "./types";

function Sidebar() {
  const titleRef = useRef<HTMLDivElement>(null);
  const itemsRef = useRef<HTMLUListElement>(null);

  // Prepare list items from GeoJSON data
  const features = (geoJsonRowData as FeatureCollection<Geometry, GeoProperties>).features;
  const listItems = features.map((f: Feature<Geometry, GeoProperties>, i: number) => {
    const properties = f.properties;
    return (
      <li key={i}>
        <h3>{properties.name}</h3>
        <p>{properties.category}</p>
        <p>{properties.address}</p>
        <a href={properties.youtube} target="_blank" rel="noopener noreferrer">YouTube</a>
      </li>
    );
  });

  // Sync padding-top of items to height of title
  useEffect(() => {
    function onTitleResize() {
      if (titleRef.current && itemsRef.current) {
        const h = titleRef.current.offsetHeight;
        itemsRef.current.style.paddingTop = `${h}px`;
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
          <h3 id="subtitle">敷嶋てとら ファンメイド<br />聖地巡礼マップ</h3>
        </div>
        <ul id="items" ref={itemsRef}>
          {features.length !== 0 ? listItems : <li>データが見つかりません</li>}
        </ul>
      </div>
    </div>
  );
}

export default Sidebar;
