#!/usr/bin/env node
// Registers the spot of a "new spot" issue (see .github/ISSUE_TEMPLATE/spot_request.yml)
// through the admin API when a maintainer comments "/approve" on it, then reports the
// result on the issue.
//
//   /approve                          register; the coordinates come from the address
//   /approve location=<lat>,<lng>     register at the given coordinates
//   /approve spot_id=<id>             add the video to a spot that is already registered
//
// The coordinates are looked up with the address search of the Geospatial Information
// Authority of Japan (国土地理院), which is free. A spot with the same name and address
// as a registered one only gets the video added.
//
// The workflow (.github/workflows/spot_request.yml) decides who may approve.
//
// Run directly with Node's built-in TypeScript support: `node process-spot-request.ts`.
import { execFileSync } from "node:child_process";

interface LatLng {
  lat: number;
  lng: number;
}

interface ApiError {
  error?: { code?: string; message?: string };
}

interface ApiSuccess {
  spot: { id: string; name: string; address: string; location: LatLng; isClosed: boolean };
  created: boolean;
  appearanceAdded: boolean;
}

type Approval = { spotId?: string; location?: LatLng };

function requireEnv(key: string): string {
  const value = process.env[key];
  if (!value) throw new Error(`Missing required env var: ${key}`);
  return value;
}

const repo = requireEnv("GITHUB_REPOSITORY");
const issueNumber = requireEnv("ISSUE_NUMBER");
const apiUrl = requireEnv("SPOT_API_URL");
const audience = requireEnv("SPOT_API_AUDIENCE");
const issueBody = process.env.ISSUE_BODY ?? "";
const commentBody = process.env.COMMENT_BODY ?? "";

function comment(body: string): void {
  execFileSync(
    "gh",
    ["issue", "comment", issueNumber, "--repo", repo, "--body-file", "-"],
    { input: body, stdio: ["pipe", "inherit", "inherit"] }
  );
}

function closeIssue(): void {
  execFileSync(
    "gh",
    ["issue", "close", issueNumber, "--repo", repo, "--reason", "completed"],
    { stdio: "inherit" }
  );
}

// Comments the reason on the issue and fails the job.
function fail(message: string): never {
  console.error(message);
  comment(`スポットを登録できませんでした。\n\n${message}`);
  process.exit(1);
}

// Reads the approval command on the first line of the comment, or returns null
// when the comment is not one (e.g. "/approved!").
function parseApproval(body: string): Approval | null {
  const firstLine = body.split(/\r?\n/, 1)[0].trim();
  if (firstLine === "/approve") return {};
  const spot = firstLine.match(/^\/approve\s+spot_id=([0-9a-fA-F-]{36})$/);
  if (spot) return { spotId: spot[1] };
  const location = firstLine.match(/^\/approve\s+location=(-?\d+(?:\.\d+)?),\s*(-?\d+(?:\.\d+)?)$/);
  if (location) return { location: { lat: Number(location[1]), lng: Number(location[2]) } };
  return null;
}

function parseSections(body: string): Record<string, string> {
  const sections: Record<string, string> = {};
  const chunks = body.split(/\r?\n### /);
  chunks.forEach((chunk, i) => {
    const text = i === 0 ? chunk.replace(/^### /, "") : chunk;
    const newlineIndex = text.indexOf("\n");
    if (newlineIndex === -1) return;
    const heading = text.slice(0, newlineIndex).trim();
    const content = text.slice(newlineIndex + 1).trim();
    sections[heading] = content;
  });
  return sections;
}

function parseYoutube(url: string): { id: string | null; timestamp: number } {
  const idMatch = url.match(/(?:v=|youtu\.be\/|\/live\/|\/shorts\/)([A-Za-z0-9_-]{6,})/);
  const timeMatch = url.match(/[?&]t=(\d+)s?/);
  return {
    id: idMatch ? idMatch[1] : null,
    timestamp: timeMatch ? parseInt(timeMatch[1], 10) : 0,
  };
}

interface GsiAddressResult {
  geometry: { coordinates: [number, number] };
}

// Strips the postal code prefix so the 国土地理院 (GSI) address search API can match it.
// The separator between the two digit groups varies by submitter: half-width
// hyphen, full-width hyphen, or a full-width minus sign are all seen in practice.
function simplifyAddress(address: string): string {
  return address.replace(/^〒?\s*\d{3}[-‐‑‒–—−－]?\d{4}\s*/, "").trim();
}

// Looks the address up, first in full and then without the building name.
async function geocode(address: string): Promise<LatLng | null> {
  const simplified = simplifyAddress(address);
  for (const query of [simplified, simplified.split(/\s+/)[0]]) {
    if (!query) continue;
    const url = `https://msearch.gsi.go.jp/address-search/AddressSearch?q=${encodeURIComponent(query)}`;
    try {
      const res = await fetch(url);
      if (!res.ok) continue;
      const results = (await res.json()) as GsiAddressResult[];
      if (results.length > 0) {
        const [lng, lat] = results[0].geometry.coordinates;
        return { lat, lng };
      }
    } catch {
      // Treated like no result: the maintainer can give the coordinates.
    }
  }
  return null;
}

// Requests the OIDC token GitHub issues to this workflow run.
async function fetchIdToken(): Promise<string> {
  const requestUrl = new URL(requireEnv("ACTIONS_ID_TOKEN_REQUEST_URL"));
  requestUrl.searchParams.set("audience", audience);
  const res = await fetch(requestUrl, {
    headers: { Authorization: `Bearer ${requireEnv("ACTIONS_ID_TOKEN_REQUEST_TOKEN")}` },
  });
  if (!res.ok) throw new Error(`Could not get an OIDC token: HTTP ${res.status}`);
  const { value } = (await res.json()) as { value?: string };
  if (!value) throw new Error("The OIDC token response has no value");
  return value;
}

const usage = [
  "- `/approve`: 住所から座標を求めて登録する",
  "- `/approve location=<緯度>,<経度>`: 指定した座標で登録する (例: `/approve location=35.6812,139.7671`)",
  "- `/approve spot_id=<スポット ID>`: 登録済みのスポットに動画だけを追加する",
].join("\n");

// 1. Read the approval comment.
const approval = parseApproval(commentBody);
if (!approval) {
  console.log("The comment is not an approval command; nothing to do.");
  comment(`承認コマンドとして解釈できませんでした。次のいずれかの形式でコメントしてください。\n\n${usage}`);
  process.exit(0);
}

// 2. Parse the issue body into the fields defined by the issue template.
const sections = parseSections(issueBody);
const name = sections["スポット名"];
const address = sections["住所"];
const youtubeLink = sections["YouTubeリンク"];
const supplementary =
  sections["補足情報"] === "_No response_" ? "" : (sections["補足情報"] ?? "");

if (!name || !address || !youtubeLink) {
  fail("Issue 本文からスポット名、住所、YouTube リンクを読み取れませんでした。テンプレートの形式を確認してください。");
}
const { id: youtubeId, timestamp } = parseYoutube(youtubeLink);
if (!youtubeId) {
  fail("YouTube リンクから動画 ID を読み取れませんでした。");
}
const isClosed = /閉業|閉店|閉鎖|営業終了/.test(supplementary);

// 3. Find the coordinates, unless the video goes to a registered spot. When the
// address cannot be found, the API still adds the video to a registered spot with
// the same name and address, and asks for coordinates otherwise.
let location = approval.location;
if (!approval.spotId && !location) {
  location = (await geocode(address)) ?? undefined;
}

// 4. Register the spot through the admin API.
const request: Record<string, unknown> = approval.spotId
  ? { spotId: approval.spotId, youtubeId, timestamp }
  : { name, address, youtubeId, timestamp, isClosed, ...(location ? { location } : {}) };

let res: Response;
try {
  res = await fetch(apiUrl, {
    method: "POST",
    headers: {
      Authorization: `Bearer ${await fetchIdToken()}`,
      "Content-Type": "application/json",
    },
    body: JSON.stringify(request),
  });
} catch (err) {
  fail(`管理 API を呼び出せませんでした: ${err instanceof Error ? err.message : String(err)}`);
}
const text = await res.text();
let payload: unknown;
try {
  payload = JSON.parse(text);
} catch {
  fail(`管理 API が JSON 以外を返しました (HTTP ${res.status})。`);
}

// 5. Report the result on the issue.
if (res.ok) {
  const { spot, created, appearanceAdded } = payload as ApiSuccess;
  const outcome = created
    ? "スポットを登録しました。座標が正しいか地図で確認してください。"
    : appearanceAdded
      ? "同じスポットが登録済みのため、動画の登場シーンだけを追加しました。"
      : "このスポットと登場シーンは登録済みです。変更はありません。";
  const { lat, lng } = spot.location;
  comment(
    [
      outcome,
      "",
      `- スポット: ${spot.name}`,
      `- 住所: ${spot.address}`,
      `- 座標: [${lat}, ${lng}](https://www.openstreetmap.org/?mlat=${lat}&mlon=${lng}#map=18/${lat}/${lng})`,
      `- 閉業: ${spot.isClosed ? "はい" : "いいえ"}`,
      `- スポット ID: \`${spot.id}\``,
      "",
      "座標や閉業状態の修正は、Actions の「Spot Update」ワークフローから行えます。",
    ].join("\n")
  );
  closeIssue();
  console.log(`${created ? "Created" : "Updated"} spot ${spot.id}`);
  process.exit(0);
}

const { error } = payload as ApiError;
if (res.status === 422 && error?.code === "location_required") {
  // Keep the issue open until a maintainer gives the coordinates.
  comment(
    [
      "住所から座標を求められなかったため、登録を保留しました。",
      "地図で座標を調べ、`/approve location=<緯度>,<経度>` とコメントしてください。",
    ].join("\n")
  );
  console.log("The address could not be located; waiting for coordinates.");
  process.exit(0);
}

fail(`管理 API がエラーを返しました (HTTP ${res.status} \`${error?.code ?? "unknown"}\`): ${error?.message ?? text.slice(0, 200)}`);
