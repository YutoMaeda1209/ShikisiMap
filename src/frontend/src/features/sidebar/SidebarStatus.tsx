import type { Profile } from "../../lib/api";
import { useAuth } from "../auth/authContext";
import { useMapData } from "../spots/mapDataContext";
import { useVisits } from "../visits/visitsContext";

function requestAccountDialog() {
  window.dispatchEvent(new CustomEvent("account:open"));
}

/** The visitor's progress and the button that opens the account dialog. */
export function VisitorStatus() {
  const { me, isLoading } = useAuth();
  const { allSpots } = useMapData();
  const { visitedSpotIds } = useVisits();

  // Closed spots can no longer be visited, so they count on neither side.
  const openSpots = allSpots.filter((s) => !s.isClosed);
  const visitedCount = openSpots.filter((s) => visitedSpotIds.has(s.id)).length;

  return (
    <>
      <span id="visitProgress">
        {visitedCount} / {openSpots.length} 件 訪問済み
      </span>
      <button id="accountButton" type="button" onClick={requestAccountDialog} disabled={isLoading}>
        {me ? me.displayName : "ログイン"}
      </button>
    </>
  );
}

/** The heading of someone's public profile. */
export function ProfileStatus({ profile }: { profile: Profile }) {
  return (
    <>
      <span id="profileName">{profile.displayName} さんの訪問済みスポット</span>
      <span>
        {profile.visitedSpotIds.length} 件 ・ <a className="internal" href="/">
          地図に戻る
        </a>
      </span>
    </>
  );
}
