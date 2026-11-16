#!/usr/bin/env node
// Registers the spot of a "new spot" issue (see .github/ISSUE_TEMPLATE/spot_request.yml)
// through the admin API when a maintainer comments "/approve" on it, then reports the
// result on the issue.
//
//   /approve                    resolve the place from the name and address
//   /approve place_id=<id>      register with the given Google place ID
//
// The workflow (.github/workflows/spot_request.yml) decides who may approve.
//
// Run directly with Node's built-in TypeScript support: `node process-spot-request.ts`.
import { execFileSync } from "node:child_process";

interface Candidate {
  placeId: string;
  name: string;
  address: string;
}

interface ApiError {
  error?: { code?: string; message?: string };
  candidates?: Candidate[];
}

interface ApiSuccess {
  spot: { id: string; name: string; address: string; placeId: string | null };
  created: boolean;
  appearanceAdded: boolean;
}

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

// Returns the place ID given in the comment, null when there is none, or
// undefined when the comment is not an approval at all (e.g. "/approved!").
function parseApproval(body: string): string | null | undefined {
  const firstLine = body.split(/\r?\n/, 1)[0].trim();
  const match = firstLine.match(/^\/approve(?:\s+place_id=(\S+))?$/);
  if (!match) return undefined;
  return match[1] ?? null;
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

// Keeps text from a third party from breaking a Markdown table.
function cell(text: string): string {
  return text.replace(/\|/g, "\\|").replace(/\r?\n/g, " ");
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

// 1. Read the approval comment.
const placeId = parseApproval(commentBody);
if (placeId === undefined) {
  console.log("The comment is not an approval command; nothing to do.");
  comment(
    "承認コマンドとして解釈できませんでした。`/approve` または `/approve place_id=<place_id>` の形式でコメントしてください。"
  );
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

// 3. Register the spot through the admin API.
const request: Record<string, unknown> = { name, address, youtubeId, timestamp, isClosed };
if (placeId) request.placeId = placeId;

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

// 4. Report the result on the issue.
if (res.ok) {
  const { spot, created, appearanceAdded } = payload as ApiSuccess;
  const outcome = created
    ? "スポットを登録しました。"
    : appearanceAdded
      ? "同じ場所のスポットが登録済みのため、動画の登場シーンだけを追加しました。"
      : "このスポットと登場シーンは登録済みです。変更はありません。";
  comment(
    [
      outcome,
      "",
      `- スポット: ${spot.name}`,
      `- 住所: ${spot.address}`,
      `- place_id: \`${spot.placeId ?? "未解決"}\``,
      `- スポット ID: \`${spot.id}\``,
    ].join("\n")
  );
  closeIssue();
  console.log(`${created ? "Created" : "Updated"} spot ${spot.id}`);
  process.exit(0);
}

const { error, candidates } = payload as ApiError;
if (res.status === 409 && error?.code === "place_not_resolved") {
  // Keep the issue open until a maintainer picks the place.
  const lines =
    candidates && candidates.length > 0
      ? [
          "名前と住所に一致する場所が複数見つかったため、登録を保留しました。",
          "正しい場所の place_id を指定して `/approve place_id=<place_id>` とコメントしてください。",
          "",
          "| 名前 | 住所 | place_id |",
          "| --- | --- | --- |",
          ...candidates.map((c) => `| ${cell(c.name)} | ${cell(c.address)} | \`${c.placeId}\` |`),
        ]
      : [
          "名前と住所に一致する場所が見つからなかったため、登録を保留しました。",
          "Google マップで place_id を調べ、`/approve place_id=<place_id>` とコメントしてください。",
        ];
  comment(lines.join("\n"));
  console.log("The place could not be resolved; waiting for a place ID.");
  process.exit(0);
}

fail(`管理 API がエラーを返しました (HTTP ${res.status} \`${error?.code ?? "unknown"}\`): ${error?.message ?? text.slice(0, 200)}`);
