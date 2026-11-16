import { useEffect, useLayoutEffect, useRef, useState } from "react";
import type { RowComponentProps } from "react-window";
import type { Spot } from "./api";
import { useLocationSelection } from "./locationSelectionContext";
import SpotCard from "./SpotCard";
import "./SpotItem.css";

interface ListItemProps {
  spots: Spot[];
  disableInlineVideo?: boolean;
  inlineVideoResetKey?: number;
}

function SpotItem({
  index,
  style,
  spots,
  disableInlineVideo = false,
  inlineVideoResetKey = 0,
}: RowComponentProps<ListItemProps>) {
  const spot = spots[index];
  const {selectedId, select} = useLocationSelection();
  const spotItemRootRef = useRef<HTMLDivElement | null>(null);
  const [isCompact, setIsCompact] = useState(false);

  useEffect(() => {
    if (selectedId !== spot.id || !spotItemRootRef.current) return;

    const spotItemRef = spotItemRootRef.current;
    spotItemRef.classList.add("blink-highlight");
    const t = setTimeout(() => spotItemRef.classList.remove("blink-highlight"), 1500);
    return () => {
      spotItemRef.classList.remove("blink-highlight");
      clearTimeout(t);
    };
  }, [selectedId, spot.id]);

  const isSelected = selectedId === spot.id;

  useLayoutEffect(() => {
    const node = spotItemRootRef.current;
    if (!node) return;
    const targetHeight = typeof style?.height === "number" ? style.height : null;
    if (targetHeight === null) return;

    let raf = 0;
    raf = requestAnimationFrame(() => {
      const isOverflowing = node.scrollHeight > targetHeight;
      setIsCompact((prev) => (prev !== isOverflowing ? isOverflowing : prev));
    });
    return () => {
      if (raf) cancelAnimationFrame(raf);
    };
  }, [style?.height, spot.id]);

  function onClickSpotItem() {
    if (selectedId === spot.id) {
      window.dispatchEvent(new CustomEvent("sidebar:request-open-sheet"));
      return;
    }
    select(spot.id);
  }

  return (
    <div className="spotItemRow" style={style}>
      <div
        ref={spotItemRootRef}
        className={`spotItem${spot.isClosed ? " closed" : ""}${isSelected ? " selected" : ""}${isCompact ? " compact-title" : ""}`}
        onClick={onClickSpotItem}
      >
        <SpotCard
          // A row is reused for another spot while scrolling; the key resets its state.
          key={spot.id}
          spot={spot}
          videoKey={`item-${inlineVideoResetKey}`}
          disableInlineVideo={disableInlineVideo}
        />
      </div>
    </div>
  );
}

export default SpotItem;
