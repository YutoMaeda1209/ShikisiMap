// Where the visit records live: in the account when signed in, otherwise in
// this browser's localStorage. Both sit behind the same interface.
import { api, apiBlob, ApiError, type Spot, type Visit } from "../../lib/api";

export type VisitInput = {
  spotId: string;
  visitedAt: string;
  memo: string | null;
};

export type VisitPatch = {
  visitedAt?: string;
  memo?: string | null;
};

export type AddResult = {
  created: Visit[];
  /** Records left out because the same spot and time were already recorded. */
  skipped: number;
};

export interface VisitStore {
  list(): Promise<Visit[]>;
  add(inputs: VisitInput[]): Promise<AddResult>;
  update(id: string, patch: VisitPatch): Promise<Visit>;
  remove(id: string): Promise<void>;
}

export type ExportFormat = "json" | "csv";

export class DuplicateVisitError extends Error {
  constructor() {
    super("another visit to the spot has the same date and time");
    this.name = "DuplicateVisitError";
  }
}

const storageKey = "shikisimap.visits";

function isVisit(value: unknown): value is Visit {
  if (typeof value !== "object" || value === null) return false;
  const v = value as Record<string, unknown>;
  return (
    typeof v.id === "string" &&
    typeof v.spotId === "string" &&
    typeof v.visitedAt === "string" &&
    !Number.isNaN(Date.parse(v.visitedAt)) &&
    (v.memo === null || typeof v.memo === "string")
  );
}

/** Reads the records kept in this browser. Damaged data reads as no records. */
export function readLocalVisits(): Visit[] {
  try {
    const parsed: unknown = JSON.parse(window.localStorage.getItem(storageKey) ?? "[]");
    return Array.isArray(parsed) ? parsed.filter(isVisit) : [];
  } catch {
    return [];
  }
}

function writeLocalVisits(visits: Visit[]): void {
  window.localStorage.setItem(storageKey, JSON.stringify(visits));
}

export function clearLocalVisits(): void {
  window.localStorage.removeItem(storageKey);
}

function sameMoment(a: string, b: string): boolean {
  return Date.parse(a) === Date.parse(b);
}

function normalizeMemo(memo: string | null | undefined): string | null {
  return memo && memo.trim() !== "" ? memo : null;
}

// Follows the server's rule: one record per spot and time.
export const localVisitStore: VisitStore = {
  async list() {
    return readLocalVisits();
  },

  async add(inputs) {
    const visits = readLocalVisits();
    const created: Visit[] = [];
    for (const input of inputs) {
      const duplicate = visits.some(
        (v) => v.spotId === input.spotId && sameMoment(v.visitedAt, input.visitedAt)
      );
      if (duplicate) continue;
      const visit: Visit = {
        id: crypto.randomUUID(),
        spotId: input.spotId,
        visitedAt: new Date(input.visitedAt).toISOString(),
        memo: normalizeMemo(input.memo),
      };
      visits.push(visit);
      created.push(visit);
    }
    writeLocalVisits(visits);
    return { created, skipped: inputs.length - created.length };
  },

  async update(id, patch) {
    const visits = readLocalVisits();
    const current = visits.find((v) => v.id === id);
    if (!current) throw new Error("no such visit");
    const next: Visit = {
      ...current,
      visitedAt: patch.visitedAt ? new Date(patch.visitedAt).toISOString() : current.visitedAt,
      memo: patch.memo === undefined ? current.memo : normalizeMemo(patch.memo),
    };
    const duplicate = visits.some(
      (v) => v.id !== id && v.spotId === next.spotId && sameMoment(v.visitedAt, next.visitedAt)
    );
    if (duplicate) throw new DuplicateVisitError();
    writeLocalVisits(visits.map((v) => (v.id === id ? next : v)));
    return next;
  },

  async remove(id) {
    writeLocalVisits(readLocalVisits().filter((v) => v.id !== id));
  },
};

export const accountVisitStore: VisitStore = {
  async list() {
    return (await api<{ visits: Visit[] }>("/me/visits")).visits;
  },

  async add(inputs) {
    const res = await api<{ visits: Visit[]; skipped: number }>("/me/visits", {
      method: "POST",
      body: inputs,
    });
    return { created: res.visits, skipped: res.skipped };
  },

  async update(id, patch) {
    try {
      return await api<Visit>(`/me/visits/${id}`, { method: "PATCH", body: patch });
    } catch (err) {
      if (err instanceof ApiError && err.code === "duplicate_visit") {
        throw new DuplicateVisitError();
      }
      throw err;
    }
  },

  async remove(id) {
    await api<void>(`/me/visits/${id}`, { method: "DELETE" });
  },
};

const mimeTypes: Record<ExportFormat, string> = {
  json: "application/json",
  csv: "text/csv",
};

/** Fetches the account's records in the format, for saving as a file. */
export function exportAccountVisits(format: ExportFormat): Promise<Blob> {
  return apiBlob("/me/visits", mimeTypes[format]);
}

// Keeps spreadsheet applications from running a cell as a formula.
function csvCell(text: string): string {
  const safe = /^[=+\-@\t\r]/.test(text) ? `'${text}` : text;
  return /[",\r\n]/.test(safe) ? `"${safe.replace(/"/g, '""')}"` : safe;
}

/** Builds the same export from the records kept in this browser. */
export function exportLocalVisits(format: ExportFormat, spots: Spot[]): Blob {
  const visits = readLocalVisits().sort((a, b) => Date.parse(b.visitedAt) - Date.parse(a.visitedAt));
  if (format === "json") {
    return new Blob([JSON.stringify({ visits }, null, 2)], { type: mimeTypes.json });
  }
  const names = new Map(spots.map((s) => [s.id, s.name]));
  const rows = [
    ["id", "spot_id", "spot_name", "visited_at", "memo"],
    ...visits.map((v) => [
      v.id,
      v.spotId,
      csvCell(names.get(v.spotId) ?? ""),
      v.visitedAt,
      csvCell(v.memo ?? ""),
    ]),
  ];
  // The byte order mark makes Excel read the file as UTF-8.
  return new Blob(["\ufeff" + rows.map((r) => r.join(",")).join("\n") + "\n"], {
    type: `${mimeTypes.csv}; charset=utf-8`,
  });
}

/** Makes the browser save the blob under the file name. */
export function downloadBlob(blob: Blob, fileName: string): void {
  const url = URL.createObjectURL(blob);
  const link = document.createElement("a");
  link.href = url;
  link.download = fileName;
  document.body.appendChild(link);
  link.click();
  link.remove();
  URL.revokeObjectURL(url);
}
