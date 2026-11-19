import { useEffect, useState } from "react";
import MapView from "../features/map/MapView";
import Sidebar from "../features/sidebar/Sidebar";
import { ProfileStatus } from "../features/sidebar/SidebarStatus";
import LocationSelectionProvider from "../features/spots/LocationSelectionProvider";
import MapDataProvider from "../features/spots/MapDataProvider";
import UrlManager from "../features/spots/UrlManager";
import ProfileVisitsProvider from "../features/visits/ProfileVisitsProvider";
import { api, ApiError, type Profile } from "../lib/api";

// Someone's public profile: the same map, limited to the spots they visited.
function ProfilePage({ userId }: { userId: string }) {
  const [profile, setProfile] = useState<Profile | null>(null);
  const [error, setError] = useState<"notFound" | "failed" | null>(null);

  useEffect(() => {
    let cancelled = false;
    api<Profile>(`/users/${userId}`)
      .then((p) => {
        if (!cancelled) setProfile(p);
      })
      .catch((err: unknown) => {
        if (cancelled) return;
        setError(err instanceof ApiError && err.status === 404 ? "notFound" : "failed");
      });
    return () => {
      cancelled = true;
    };
  }, [userId]);

  if (error) {
    return (
      <div id="pageMessage">
        <p>
          {error === "notFound"
            ? "このプロフィールは存在しないか、公開されていません。"
            : "プロフィールを読み込めませんでした。"}
        </p>
        <a className="internal" href="/">
          地図に戻る
        </a>
      </div>
    );
  }
  if (!profile) {
    return (
      <div id="pageMessage">
        <p>読み込み中…</p>
      </div>
    );
  }

  return (
    <LocationSelectionProvider>
      <ProfileVisitsProvider visitedSpotIds={profile.visitedSpotIds}>
        <MapDataProvider onlySpotIds={profile.visitedSpotIds}>
          <div id="sidebarContainer">
            <Sidebar status={<ProfileStatus profile={profile} />} />
          </div>
          <MapView />
          <UrlManager />
        </MapDataProvider>
      </ProfileVisitsProvider>
    </LocationSelectionProvider>
  );
}

export default ProfilePage;
