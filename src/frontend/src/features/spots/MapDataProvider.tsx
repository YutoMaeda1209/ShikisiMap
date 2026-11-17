import { useEffect, useState, type ReactNode } from "react";
import { api, type Spot } from "../../lib/api";
import { MapDataContext, type VisitFilter } from "./mapDataContext";
import { useVisits } from "../visits/visitsContext";

type MapDataProviderProps = {
  /** Limits the page to these spots. A profile page shows only the visited ones. */
  onlySpotIds?: string[];
  children: ReactNode;
};

function MapDataProvider({ onlySpotIds, children }: MapDataProviderProps) {
  const { visitedSpotIds, accountRevision } = useVisits();
  const [spots, setSpots] = useState<Spot[]>([]);
  const [status, setStatus] = useState<"loading" | "ready" | "error">("loading");
  const [attempt, setAttempt] = useState(0);
  const [query, setQuery] = useState("");
  const [visitFilter, setVisitFilter] = useState<VisitFilter>("all");

  useEffect(() => {
    let cancelled = false;
    api<{ spots: Spot[] }>("/spots")
      .then((res) => {
        if (cancelled) return;
        setSpots(res.spots);
        setStatus("ready");
      })
      .catch((err: unknown) => {
        console.error("Could not load the spots:", err);
        if (!cancelled) setStatus("error");
      });
    return () => {
      cancelled = true;
    };
    // Loading again after the account's visits changed updates the visitor counts.
  }, [attempt, accountRevision]);

  const reload = () => {
    setStatus("loading");
    setAttempt((n) => n + 1);
  };

  const only = onlySpotIds ? new Set(onlySpotIds) : null;
  const allSpots = only ? spots.filter((s) => only.has(s.id)) : spots;

  const q = query.trim().toLowerCase();
  const shownSpots = allSpots.filter((s) => {
    if (q && !s.name.toLowerCase().includes(q) && !s.address.toLowerCase().includes(q)) {
      return false;
    }
    if (visitFilter === "visited") return visitedSpotIds.has(s.id);
    if (visitFilter === "unvisited") return !visitedSpotIds.has(s.id);
    return true;
  });

  return (
    <MapDataContext.Provider
      value={{ allSpots, shownSpots, status, reload, setQuery, visitFilter, setVisitFilter }}
    >
      {children}
    </MapDataContext.Provider>
  );
}

export default MapDataProvider;
