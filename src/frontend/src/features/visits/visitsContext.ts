import { createContext, useContext } from "react";
import type { Visit } from "../../lib/api";
import type { AddResult, VisitInput, VisitPatch } from "./visitStore";

export type VisitsContextType = {
  /** The visitor's own records, newest first. Empty on a profile page. */
  visits: Visit[];
  /** Spots with at least one visit. */
  visitedSpotIds: ReadonlySet<string>;
  /** True on someone else's profile, where nothing can be recorded. */
  readOnly: boolean;
  /** Whether the records are kept in the account or only in this browser. */
  storage: "account" | "local";
  /** Grows whenever the account's records change, which changes the visitor counts. */
  accountRevision: number;
  addVisit: (input: VisitInput) => Promise<AddResult>;
  updateVisit: (id: string, patch: VisitPatch) => Promise<void>;
  removeVisit: (id: string) => Promise<void>;
  /** Records in this browser that a signed-in user can still import. */
  localImportCount: number;
  importLocal: () => Promise<AddResult>;
  discardLocal: () => void;
};

export const VisitsContext = createContext<VisitsContextType | undefined>(undefined);

export function useVisits(): VisitsContextType {
  const ctx = useContext(VisitsContext);
  if (!ctx) throw new Error("useVisits must be used within a VisitsProvider");
  return ctx;
}

/** Opens the dialog for recording and reviewing the visits to a spot. */
export function requestVisitDialog(spotId: string): void {
  window.dispatchEvent(new CustomEvent("visit:open", { detail: spotId }));
}
