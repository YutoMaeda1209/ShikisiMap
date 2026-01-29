import { useRef } from "react";
import type { Feature, Geometry } from "geojson";
import LiteYouTubeEmbed from "react-lite-youtube-embed";
import "react-lite-youtube-embed/dist/LiteYouTubeEmbed.css";
import type { GeoProperties } from "./mapData";
import "./BottomSheet.css";

type BottomSheetProps = {
  feature: Feature<Geometry, GeoProperties>;
  isOpen: boolean;
  onRequestClose: () => void;
};

function BottomSheet({ feature, isOpen, onRequestClose }: BottomSheetProps) {
  const touchStartYRef = useRef<number | null>(null);
  const touchDeltaYRef = useRef(0);

  function handleTouchStart(event: React.TouchEvent<HTMLDivElement>) {
    if (!isOpen) return;
    touchStartYRef.current = event.touches[0]?.clientY ?? null;
    touchDeltaYRef.current = 0;
  }

  function handleTouchMove(event: React.TouchEvent<HTMLDivElement>) {
    if (!isOpen) return;
    if (touchStartYRef.current === null) return;
    const currentY = event.touches[0]?.clientY ?? touchStartYRef.current;
    touchDeltaYRef.current = Math.max(0, currentY - touchStartYRef.current);
  }

  function handleTouchEnd() {
    if (!isOpen) return;
    const shouldClose = touchDeltaYRef.current >= 40;
    touchStartYRef.current = null;
    touchDeltaYRef.current = 0;
    if (shouldClose) {
      onRequestClose();
    }
  }

  return (
    <div
      className={`bottomSheet ${isOpen ? "open" : "closed"}`}
      role="dialog"
      aria-modal="false"
    >
      <div
        className="bottomSheetHeader"
        onTouchStart={handleTouchStart}
        onTouchMove={handleTouchMove}
        onTouchEnd={handleTouchEnd}
      >
        <span className="bottomSheetTitle">{feature.properties.name}</span>
        <button
          type="button"
          className="bottomSheetClose"
          onClick={onRequestClose}
          aria-label="閉じる"
        >
          ×
        </button>
      </div>
      <div className="bottomSheetBody">
        <LiteYouTubeEmbed
          id={feature.properties.youtubeId}
          title={feature.properties.name}
          lazyLoad={true}
          params={`?start=${feature.properties.timestamp}`}
        />
        <a
          className="address"
          href={`https://www.google.com/maps/search/${feature.properties.name} ${feature.properties.address}`}
          target="_blank"
          rel="noopener noreferrer"
        >
          {feature.properties.address}
        </a>
      </div>
    </div>
  );
}

export default BottomSheet;
