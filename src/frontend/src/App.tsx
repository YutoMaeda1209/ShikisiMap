import { useEffect, useState } from "react";
import { api, ApiError, type Profile } from "./api";
import "./App.css";
import AuthProvider from "./AuthProvider";
import DialogHost from "./DialogHost";
import LocationSelectionProvider from "./LocationSelectionProvider";
import MapView from "./Map";
import MapDataProvider from "./MapDataProvider";
import ProfileVisitsProvider from "./ProfileVisitsProvider";
import { parseRoute } from "./route";
import Sidebar from "./Sidebar";
import { ProfileStatus, VisitorStatus } from "./SidebarStatus";
import UrlManager from "./UrlManager";
import VisitsProvider from "./VisitsProvider";

// The map with the visitor's own visit records.
function MapPage() {
  return (
    <AuthProvider>
      <LocationSelectionProvider>
        <VisitsProvider>
          <MapDataProvider>
            <div id="sidebarContainer">
              <Sidebar status={<VisitorStatus />} />
            </div>
            <MapView />
            <DialogHost />
            <UrlManager />
          </MapDataProvider>
        </VisitsProvider>
      </LocationSelectionProvider>
    </AuthProvider>
  );
}

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

function App() {
  const route = parseRoute(window.location.pathname);
  return (
    <div id="app">
      {route.page === "profile" ? <ProfilePage userId={route.userId} /> : <MapPage />}
    </div>
  );
}

export default App;
