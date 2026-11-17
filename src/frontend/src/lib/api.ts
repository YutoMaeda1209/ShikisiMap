// Types and helpers for the backend API (see docs/spec.md).

export type Appearance = {
  id: string;
  youtubeId: string;
  /** Position in the video, in seconds. */
  timestamp: number;
};

export type Spot = {
  id: string;
  placeId: string | null;
  name: string;
  address: string;
  location: { lat: number; lng: number };
  isClosed: boolean;
  visitorCount: number;
  /** Ordered from the most recently registered one. */
  appearances: Appearance[];
};

export type Me = {
  id: string;
  email: string;
  displayName: string;
  profilePublic: boolean;
};

export type Profile = {
  id: string;
  displayName: string;
  visitedSpotIds: string[];
};

export type Visit = {
  id: string;
  spotId: string;
  /** ISO 8601 date and time. */
  visitedAt: string;
  memo: string | null;
};

export class ApiError extends Error {
  readonly status: number;
  readonly code: string;

  constructor(status: number, code: string, message: string) {
    super(message);
    this.name = "ApiError";
    this.status = status;
    this.code = code;
  }
}

type RequestOptions = {
  method?: "GET" | "POST" | "PATCH" | "DELETE";
  body?: unknown;
  accept?: string;
};

async function request(path: string, options: RequestOptions): Promise<Response> {
  const headers: Record<string, string> = {
    Accept: options.accept ?? "application/json",
  };
  if (options.body !== undefined) {
    headers["Content-Type"] = "application/json";
  }
  const res = await fetch(`/api${path}`, {
    method: options.method ?? "GET",
    headers,
    body: options.body === undefined ? undefined : JSON.stringify(options.body),
    // The session cookie is only sent to the same origin.
    credentials: "same-origin",
  });
  if (res.ok) return res;

  let code = "unknown";
  let message = `HTTP ${res.status}`;
  try {
    const payload = (await res.json()) as { error?: { code?: string; message?: string } };
    code = payload.error?.code ?? code;
    message = payload.error?.message ?? message;
  } catch {
    // Not the API's error format, e.g. an error page of the proxy.
  }
  throw new ApiError(res.status, code, message);
}

/** Calls the API and returns the JSON body, or undefined for 204 No Content. */
export async function api<T>(path: string, options: RequestOptions = {}): Promise<T> {
  const res = await request(path, options);
  if (res.status === 204) return undefined as T;
  return (await res.json()) as T;
}

/** Calls the API and returns the body as a file to download. */
export async function apiBlob(path: string, accept: string): Promise<Blob> {
  const res = await request(path, { accept });
  return res.blob();
}

/**
 * Link to the place on Google Maps. With a place ID, Google opens that exact
 * place; the query is the fallback and what is shown while loading.
 */
export function googleMapsUrl(spot: Pick<Spot, "name" | "address" | "placeId">): string {
  const query = encodeURIComponent(`${spot.name} ${spot.address}`);
  const url = `https://www.google.com/maps/search/?api=1&query=${query}`;
  return spot.placeId ? `${url}&query_place_id=${encodeURIComponent(spot.placeId)}` : url;
}
