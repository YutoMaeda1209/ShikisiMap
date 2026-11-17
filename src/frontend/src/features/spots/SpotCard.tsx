import { useState, type Ref } from "react";
import LiteYouTubeEmbed from "react-lite-youtube-embed";
import "react-lite-youtube-embed/dist/LiteYouTubeEmbed.css";
import { googleMapsUrl, type Spot } from "../../lib/api";
import "./SpotCard.css";
import { requestVisitDialog, useVisits } from "../visits/visitsContext";

type SpotCardProps = {
  spot: Spot;
  /** Changing it remounts the player, which stops a playing video. */
  videoKey: string;
  /** The title is drawn by the surrounding component, as in the bottom sheet. */
  hideTitle?: boolean;
  disableInlineVideo?: boolean;
  contentRef?: Ref<HTMLDivElement>;
};

// The content of a spot: title, video, address and the row with the visit
// button. Every card has the same rows so that the list can use one height.
function SpotCard({ spot, videoKey, hideTitle = false, disableInlineVideo = false, contentRef }: SpotCardProps) {
  const { visits, visitedSpotIds, readOnly } = useVisits();
  // Appearances are ordered newest first, so 0 is the latest video.
  const [appearanceIndex, setAppearanceIndex] = useState(0);

  const count = spot.appearances.length;
  const index = Math.min(appearanceIndex, Math.max(0, count - 1));
  const appearance = count > 0 ? spot.appearances[index] : null;
  const isVisited = visitedSpotIds.has(spot.id);
  const visitCount = visits.filter((v) => v.spotId === spot.id).length;

  function showAppearance(event: React.MouseEvent, next: number) {
    // The card itself is clickable; these buttons must not select the spot.
    event.stopPropagation();
    setAppearanceIndex((next + count) % count);
  }

  return (
    <div className="spotContent" ref={contentRef}>
      {hideTitle ? null : (
        <span className="spotTitle">
          <span>{spot.name}</span>
          {spot.isClosed ? <span className="closedBadge">閉業</span> : null}
        </span>
      )}
      <div className={`ytLiteWrapper${disableInlineVideo ? " disabled" : ""}`}>
        {appearance ? (
          <LiteYouTubeEmbed
            key={`${videoKey}-${appearance.id}`}
            id={appearance.youtubeId}
            title={spot.name}
            lazyLoad={true}
            params={`?start=${appearance.timestamp}`}
          />
        ) : (
          <div className="noVideo">動画はありません</div>
        )}
      </div>
      <a
        className="address"
        href={googleMapsUrl(spot)}
        target="_blank"
        rel="noopener noreferrer"
        onClick={(event) => event.stopPropagation()}
      >
        {spot.address}
      </a>
      <div className="spotMeta">
        <span className="visitorCount" title="訪問を記録したユーザーの数">
          {spot.visitorCount} 人が訪問
        </span>
        {count > 1 ? (
          <span className="appearanceSwitcher">
            <button type="button" aria-label="前の動画" onClick={(e) => showAppearance(e, index - 1)}>
              ‹
            </button>
            <span aria-live="polite">
              動画 {index + 1} / {count}
            </span>
            <button type="button" aria-label="次の動画" onClick={(e) => showAppearance(e, index + 1)}>
              ›
            </button>
          </span>
        ) : null}
        {readOnly ? (
          isVisited ? <span className="visitedLabel">訪問済み</span> : null
        ) : (
          <button
            type="button"
            className={`visitButton${isVisited ? " visited" : ""}`}
            onClick={(event) => {
              event.stopPropagation();
              requestVisitDialog(spot.id);
            }}
          >
            {isVisited ? `訪問済み${visitCount > 1 ? ` (${visitCount})` : ""}` : "訪問を記録"}
          </button>
        )}
      </div>
    </div>
  );
}

export default SpotCard;
