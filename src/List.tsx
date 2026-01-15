import type { Feature, FeatureCollection, Geometry } from "geojson";
import { useEffect, useRef } from "react";
import "./List.css";
import logo from "./assets/img/logo.webp";
import geoJsonRowData from "./data.json";
import type { GeoProperties } from "./types";

function List() {
  const titleRef = useRef<HTMLDivElement | null>(null);
  const itemsRef = useRef<HTMLUListElement | null>(null);

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
    <div id="listComponent">
      <div id="listContent">
        <div id="title" ref={titleRef}>
          <img src={logo} alt="Logo" />
        </div>
        <ul id="items" ref={itemsRef}>
          {features.length !== 0 ? listItems : <li>データが見つかりません</li>}
        </ul>
      </div>
    </div>
  );
}

export default List;
