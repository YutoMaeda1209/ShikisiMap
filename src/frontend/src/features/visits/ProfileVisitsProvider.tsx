import type { ReactNode } from "react";
import { VisitsContext } from "./visitsContext";

function notAvailable(): never {
  throw new Error("visits cannot be changed on a profile page");
}

// On a public profile, "visited" means visited by the profile's owner, and
// nothing can be recorded.
function ProfileVisitsProvider({
  visitedSpotIds,
  children,
}: {
  visitedSpotIds: string[];
  children: ReactNode;
}) {
  return (
    <VisitsContext.Provider
      value={{
        visits: [],
        visitedSpotIds: new Set(visitedSpotIds),
        readOnly: true,
        storage: "account",
        accountRevision: 0,
        addVisit: notAvailable,
        updateVisit: notAvailable,
        removeVisit: notAvailable,
        localImportCount: 0,
        importLocal: notAvailable,
        discardLocal: notAvailable,
      }}
    >
      {children}
    </VisitsContext.Provider>
  );
}

export default ProfileVisitsProvider;
