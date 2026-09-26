import type { Feature, Geometry } from "geojson";
import { useRef } from "react";
import LiteYouTubeEmbed from "react-lite-youtube-embed";
import "react-lite-youtube-embed/dist/LiteYouTubeEmbed.css";
import type { GeoProperties } from "../mapDataContext";

type BottomSheetProps = {
  feature: Feature<Geometry, GeoProperties>;
  isOpen: boolean;
  onRequestClose: () => void;
};

const externalLinkClass =
  "block w-full truncate text-(length:--font-sm) leading-[1.2] text-[#333] after:ml-[5px] after:font-[bootstrap-icons] after:font-black after:content-['\\f1c5']";

function BottomSheet({ feature, isOpen, onRequestClose }: BottomSheetProps) {
  const touchStartYRef = useRef<number | null>(null);
  const touchDeltaYRef = useRef(0);

  function handleTouchStart(event: React.TouchEvent<HTMLDivElement>) {
    if (!isOpen) return;
    touchStartYRef.current = event.touches[0]?.clientY ?? null;
    touchDeltaYRef.current = 0;
  }

  function handleTouchMove(event: React.TouchEvent<HTMLDivElement>) {
    if (!isOpen || touchStartYRef.current === null) return;
    const currentY = event.touches[0]?.clientY ?? touchStartYRef.current;
    touchDeltaYRef.current = Math.max(0, currentY - touchStartYRef.current);
  }

  function handleTouchEnd() {
    if (!isOpen) return;
    const shouldClose = touchDeltaYRef.current >= 40;
    touchStartYRef.current = null;
    touchDeltaYRef.current = 0;
    if (shouldClose) onRequestClose();
  }

  return (
    <div
      className={[
        "fixed bottom-0 right-0 z-900 flex h-auto max-h-[60dvh] w-[min(680px,100%)] origin-bottom flex-col overflow-hidden rounded-t-2xl bg-white px-4 py-3 shadow-[0_-6px_20px_rgba(0,0,0,0.2)] transition-[transform,opacity] duration-220 [--sheet-header-height:48px]",
        isOpen
          ? "pointer-events-auto animate-[bottom-sheet-in_220ms_ease] opacity-100"
          : "pointer-events-none translate-y-[12%] opacity-0",
      ].join(" ")}
      role="dialog"
      aria-modal="false"
    >
      <div className="mb-4 flex min-h-(--sheet-header-height) shrink-0 flex-col gap-1.5 border-b border-dashed border-[#ccc] pt-1 pb-3.5">
        <div
          className="flex shrink-0 items-center justify-between gap-2"
          onTouchStart={handleTouchStart}
          onTouchMove={handleTouchMove}
          onTouchEnd={handleTouchEnd}
        >
          <span className="truncate text-(length:--font-lg) font-bold">
            {feature.properties.name}
          </span>
          <button
            type="button"
            className="flex flex-col relative h-8 w-8 cursor-pointer rounded-lg border border-[#ccc] bg-white text-xl text-[#333]"
            onClick={onRequestClose}
            aria-label="閉じる"
          >
            ×
          </button>
        </div>
        <a
          className={externalLinkClass}
          href={`https://www.google.com/maps/search/${feature.properties.name} ${feature.properties.address}`}
          target="_blank"
          rel="noopener noreferrer"
        >
          {feature.properties.address}
        </a>
      </div>
      <div className="flex min-h-0 flex-1 flex-col gap-2 overflow-y-auto [-webkit-overflow-scrolling:touch] [&_.yt-lite]:m-0 [&_.yt-lite]:max-w-full rounded-lg">
        <LiteYouTubeEmbed
          id={feature.properties.youtubeId}
          title={feature.properties.name}
          lazyLoad={true}
          params={`?start=${feature.properties.timestamp}`}
        />
      </div>
    </div>
  );
}

export default BottomSheet;
