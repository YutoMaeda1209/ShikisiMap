import type { Feature, Point } from "geojson";
import { useRef } from "react";
import LiteYouTubeEmbed from "react-lite-youtube-embed";
import "react-lite-youtube-embed/dist/LiteYouTubeEmbed.css";
import type { RowComponentProps } from "react-window";
import { useLocationSelection } from "./locationSelectionContext";
import type { GeoProperties } from "./mapDataContext";

export interface ListItemProps {
  features: Feature<Point, GeoProperties>[];
  disableInlineVideo?: boolean;
  inlineVideoResetKey?: number;
  focusMapOnSelect?: boolean;
}

const contentClass =
  "p-2 flex w-full flex-col gap-2 [@media(max-height:520px)]:gap-1 [@media(max-height:520px)]:py-0 [&_.yt-lite]:mx-auto [&_.yt-lite]:w-full [&_.yt-lite]:max-w-[clamp(260px,80vw,360px)] [&_.yt-lite]:overflow-hidden [&_.yt-lite]:rounded-[10px] [@media(max-height:520px)]:[&_.yt-lite]:aspect-[21/9]";

const externalLinkClass =
  "block w-full truncate text-(length:--font-sm) leading-[1.2] text-[#333] after:ml-[5px] after:font-[bootstrap-icons] after:font-black after:content-['\\f1c5'] font-sm";

function SpotItem({
  ariaAttributes,
  index,
  style,
  features,
  disableInlineVideo = false,
  inlineVideoResetKey = 0,
  focusMapOnSelect = false,
}: RowComponentProps<ListItemProps>) {
  const feature = features[index];
  const properties = feature.properties;
  const isClosed = properties.isClosed;
  const { selectedId, select } = useLocationSelection();
  const rootRef = useRef<HTMLDivElement | null>(null);
  const isSelected = selectedId === feature.id;

  function selectSpot() {
    if (isClosed) return;
    if (focusMapOnSelect) {
      window.dispatchEvent(
        new CustomEvent("map:focus-spot", {
          detail: {
            id: feature.id as string,
            coordinates: feature.geometry.coordinates as [number, number],
          },
        }),
      );
    }
    if (selectedId === feature.id) {
      window.dispatchEvent(new CustomEvent("sidebar:request-open-sheet"));
      return;
    }
    select(feature.id as string);
  }

  const itemClass = [
    "rounded-[15px] hover:cursor-pointer box-border",
    isClosed ? "bg-[#ccc]" : "",
    isSelected
      ? "border-[#ff6b00]/50 bg-[#ff6b00]/15 border-2"
      : "bg-[#ccc]/20 border-[#ccc]/80 hover:border-1 hover:bg-[#ccc]/70",
  ]
    .filter(Boolean)
    .join(" ");

  const titleClass = [
    "flex items-center gap-2 truncate font-bold leading-[1.1] justify-between text-(length:--font-lg)",
    isClosed ? "relative" : "",
  ]
    .filter(Boolean)
    .join(" ");

  return (
    <div {...ariaAttributes} style={style} className="top-0 box-border py-1">
      <div
        ref={rootRef}
        className={`${itemClass} flex h-full flex-col justify-center`}
        onClick={selectSpot}
      >
        <div className={contentClass}>
          <div
            className={`flex flex-col gap-2 py-1.5 px-0.5 border-b ${isSelected ? "border-[#ff6b00]/50" : "border-[#ccc]/70"}`}
          >
            <span className={titleClass}>
              <span className="truncate pb-0.5">{properties.name}</span>
              {isClosed ? (
                <span className="rounded-sm bg-[#c0392b] px-2 py-1 text-center text-(length:--font-sm) text-white">
                  閉業
                </span>
              ) : null}
            </span>
            {isSelected ? (
              <a
                className={`${externalLinkClass} underline`}
                href={`https://www.google.com/maps/search/${properties.name} ${properties.address}`}
                target="_blank"
                rel="noopener noreferrer"
              >
                {properties.address}
              </a>
            ) : (
              <p className={externalLinkClass}>{properties.address}</p>
            )}
          </div>
          <div
            className={
              disableInlineVideo ? "[&_.yt-lite]:pointer-events-none" : ""
            }
          >
            <LiteYouTubeEmbed
              key={`item-${feature.id}-${inlineVideoResetKey}`}
              id={properties.youtubeId}
              title={properties.name}
              lazyLoad={true}
              params={`?start=${properties.timestamp}`}
            />
          </div>
        </div>
      </div>
    </div>
  );
}

export default SpotItem;
