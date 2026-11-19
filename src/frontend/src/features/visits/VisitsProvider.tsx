import { useEffect, useState, type ReactNode } from "react";
import type { Visit } from "../../lib/api";
import { useAuth } from "../auth/authContext";
import {
  accountVisitStore,
  clearLocalVisits,
  localVisitStore,
  readLocalVisits,
  type VisitInput,
  type VisitPatch,
} from "./visitStore";
import { VisitsContext } from "./visitsContext";

function newestFirst(visits: Visit[]): Visit[] {
  return [...visits].sort((a, b) => Date.parse(b.visitedAt) - Date.parse(a.visitedAt));
}

// Holds the visitor's records. They live in the account when signed in and
// in this browser otherwise; signing in offers to import the latter.
function VisitsProvider({ children }: { children: ReactNode }) {
  const { me, isLoading: isAuthLoading } = useAuth();
  const userId = me?.id ?? null;
  const store = userId ? accountVisitStore : localVisitStore;

  const [visits, setVisits] = useState<Visit[]>([]);
  const [localCount, setLocalCount] = useState(() => readLocalVisits().length);
  const [accountRevision, setAccountRevision] = useState(0);

  const noteAccountChange = () => {
    if (userId) setAccountRevision((n) => n + 1);
  };

  useEffect(() => {
    if (isAuthLoading) return;
    let cancelled = false;
    const source = userId ? accountVisitStore : localVisitStore;
    source
      .list()
      .then((list) => {
        if (!cancelled) setVisits(newestFirst(list));
      })
      .catch((err: unknown) => {
        console.error("Could not load the visits:", err);
        if (!cancelled) setVisits([]);
      });
    return () => {
      cancelled = true;
    };
  }, [isAuthLoading, userId]);

  const syncLocalCount = () => {
    setLocalCount(readLocalVisits().length);
  };

  const addVisit = async (input: VisitInput) => {
    const result = await store.add([input]);
    setVisits((prev) => newestFirst([...prev, ...result.created]));
    syncLocalCount();
    noteAccountChange();
    return result;
  };

  const updateVisit = async (id: string, patch: VisitPatch) => {
    const updated = await store.update(id, patch);
    setVisits((prev) => newestFirst(prev.map((v) => (v.id === id ? updated : v))));
  };

  const removeVisit = async (id: string) => {
    await store.remove(id);
    setVisits((prev) => prev.filter((v) => v.id !== id));
    syncLocalCount();
    noteAccountChange();
  };

  // Sends every record of this browser to the account. The server drops the
  // ones it already has, so importing twice is harmless.
  const importLocal = async () => {
    const local = readLocalVisits();
    const result = await accountVisitStore.add(
      local.map((v) => ({ spotId: v.spotId, visitedAt: v.visitedAt, memo: v.memo }))
    );
    clearLocalVisits();
    syncLocalCount();
    setVisits((prev) => newestFirst([...prev, ...result.created]));
    noteAccountChange();
    return result;
  };

  const discardLocal = () => {
    clearLocalVisits();
    syncLocalCount();
  };

  const visitedSpotIds = new Set(visits.map((v) => v.spotId));

  return (
    <VisitsContext.Provider
      value={{
        visits,
        visitedSpotIds,
        readOnly: false,
        storage: userId ? "account" : "local",
        accountRevision,
        addVisit,
        updateVisit,
        removeVisit,
        localImportCount: userId ? localCount : 0,
        importLocal,
        discardLocal,
      }}
    >
      {children}
    </VisitsContext.Provider>
  );
}

export default VisitsProvider;
