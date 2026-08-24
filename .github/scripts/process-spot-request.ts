#!/usr/bin/env node
// Parses a "new spot" issue (see .github/ISSUE_TEMPLATE/spot_request.yml), appends the
// spot to src/data.json on a feature branch, and opens a PR against dev once every
// related issue (including sub-issues of a shared parent issue) has been processed.
// See .github/CONTRIBUTING.md for the branch/commit/PR conventions this follows.
//
// Run directly with Node's built-in TypeScript support: `node process-spot-request.ts`.
import type { Feature, FeatureCollection, Point } from "geojson";
import { execFileSync } from "node:child_process";
import { randomUUID } from "node:crypto";
import { readFileSync, writeFileSync } from "node:fs";

interface SpotProperties {
  name: string;
  address: string;
  isClosed: boolean;
  youtubeId: string;
  timestamp: number;
}

interface GhIssue {
  number: number;
}

function requireEnv(key: string): string {
  const value = process.env[key];
  if (!value) throw new Error(`Missing required env var: ${key}`);
  return value;
}

const repo = requireEnv("GITHUB_REPOSITORY");
const issueNumber = requireEnv("ISSUE_NUMBER");
const issueBody = process.env.ISSUE_BODY ?? "";

function git(args: string[]): string {
  return execFileSync("git", args, { encoding: "utf8" }).trim();
}

function gh(args: string[]): string {
  return execFileSync("gh", args, { encoding: "utf8" }).trim();
}

function ghMaybe(args: string[]): { ok: true; out: string } | { ok: false } {
  try {
    return { ok: true, out: execFileSync("gh", args, { encoding: "utf8" }) };
  } catch {
    return { ok: false };
  }
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
  const idMatch = url.match(/(?:v=|youtu\.be\/)([A-Za-z0-9_-]{6,})/);
  const timeMatch = url.match(/[?&]t=(\d+)s?/);
  return {
    id: idMatch ? idMatch[1] : null,
    timestamp: timeMatch ? parseInt(timeMatch[1], 10) : 0,
  };
}

interface GsiAddressResult {
  geometry: { coordinates: [number, number] };
}

// Strips the postal code and any trailing building name/floor (usually
// separated by a space) so the 国土地理院 (GSI) address search API can match it.
// The separator between the two digit groups varies by submitter: half-width
// hyphen, full-width hyphen, or a full-width minus sign are all seen in practice.
function simplifyAddress(address: string): string {
  return address.replace(/^〒?\s*\d{3}[-‐‑‒–—−－]?\d{4}\s*/, "").trim();
}

async function geocode(address: string): Promise<{ lat: number; lon: number } | null> {
  const candidates = [simplifyAddress(address).split(/\s+/)[0], simplifyAddress(address)];
  for (const query of candidates) {
    if (!query) continue;
    const url = `https://msearch.gsi.go.jp/address-search/AddressSearch?q=${encodeURIComponent(query)}`;
    const res = await fetch(url);
    if (!res.ok) continue;
    const results = (await res.json()) as GsiAddressResult[];
    if (results.length > 0) {
      const [lon, lat] = results[0].geometry.coordinates;
      return { lat, lon };
    }
  }
  return null;
}

// 1. Parse the issue body into the fields defined by the issue template.
const sections = parseSections(issueBody);
const name = sections["スポット名"];
const address = sections["住所"];
const youtubeLink = sections["YouTubeリンク"];
const supplementary =
  sections["補足情報"] === "_No response_" ? "" : (sections["補足情報"] ?? "");

if (!name || !address || !youtubeLink) {
  console.error("Required fields are missing or malformed; aborting without changes.");
  process.exit(1);
}

const coords = await geocode(address);
if (!coords) {
  console.error(`Could not geocode the address "${address}"; aborting without changes.`);
  process.exit(1);
}
const { lat, lon } = coords;

const { id: youtubeId, timestamp } = parseYoutube(youtubeLink);
if (!youtubeId) {
  console.error("Could not extract a YouTube video ID from the link; aborting.");
  process.exit(1);
}

const isClosed = /閉業|閉店|閉鎖|営業終了/.test(supplementary);

// 2. Is this issue a sub-issue of a parent (batch) issue?
let parentNumber: string | null = null;
const parentRes = ghMaybe(["api", `repos/${repo}/issues/${issueNumber}/parent`]);
if (parentRes.ok) {
  parentNumber = String((JSON.parse(parentRes.out) as GhIssue).number);
}

// 3. Branch: one shared branch per parent issue, otherwise one per issue.
const branch = parentNumber
  ? `feat/${parentNumber}-spot-request`
  : `feat/${issueNumber}-spot-request`;

const remoteHasBranch = git(["ls-remote", "--heads", "origin", branch]) !== "";
if (remoteHasBranch) {
  git(["fetch", "origin", branch]);
  git(["checkout", "-B", branch, `origin/${branch}`]);
} else {
  git(["checkout", "-b", branch, "origin/dev"]);
}

// 4. Append the new spot to the GeoJSON FeatureCollection.
const dataPath = "src/data.json";
const data = JSON.parse(readFileSync(dataPath, "utf8")) as FeatureCollection<
  Point,
  SpotProperties
>;
const feature: Feature<Point, SpotProperties> = {
  type: "Feature",
  properties: { name, address, isClosed, youtubeId, timestamp },
  geometry: { type: "Point", coordinates: [lon, lat] },
  id: randomUUID(),
};
data.features.push(feature);
writeFileSync(dataPath, JSON.stringify(data, null, 2) + "\n");

// 5. Commit and push (never directly to main/dev — always a feat/ branch).
git(["add", dataPath]);
git([
  "-c",
  "user.name=github-actions[bot]",
  "-c",
  "user.email=github-actions[bot]@users.noreply.github.com",
  "commit",
  "-m",
  `Add ${name}`,
  "-m",
  `Related to #${issueNumber}`,
]);
git(["push", "origin", branch]);

// 6. Decide whether every related issue has landed yet.
let issuesForPr: string[] = [issueNumber];
let allDone = true;
if (parentNumber) {
  const subIssues = JSON.parse(
    gh(["api", `repos/${repo}/issues/${parentNumber}/sub_issues`])
  ) as GhIssue[];
  issuesForPr = subIssues.map((issue) => String(issue.number));
  const commitMessages = git(["log", branch, "--format=%B"]);
  allDone = issuesForPr.every((n) => commitMessages.includes(`Related to #${n}`));
}

if (!allDone) {
  console.log("Not every sub-issue has been processed yet; skipping PR creation for now.");
  process.exit(0);
}

const existingPrs = JSON.parse(
  gh(["pr", "list", "--repo", repo, "--head", branch, "--state", "open", "--json", "number"])
) as GhIssue[];
if (existingPrs.length > 0) {
  console.log(`A PR already exists for ${branch} (#${existingPrs[0].number}); skipping.`);
  process.exit(0);
}

// 7. Open the PR against dev, following .github/pull_request_template.md.
const title = parentNumber ? `Add new spots (#${parentNumber})` : `Add spot: ${name}`;
const body = [
  "# 概要 / Summary",
  "スポットリクエストIssueの内容を `src/data.json` に反映しました。",
  "",
  "## 関連Issue / Related Issue(s)",
  ...issuesForPr.map((n) => `- #${n}`),
  "",
  "## 変更内容 / Changes",
  `- \`src/data.json\` に${issuesForPr.length}件のスポット情報を追加`,
  "",
  "## チェックリスト / Checklist",
  "- [x] 文法エラーがないことを確認しました。",
  "- [ ] 動作確認を行いました。",
  "- [x] タイトルと説明が適切であることを確認しました。",
  "- [x] 関連するIssueをリンクしました。",
].join("\n");

const bodyFile = "/tmp/spot-request-pr-body.md";
writeFileSync(bodyFile, body);

const reviewers = (process.env.REVIEWERS ?? "")
  .split(",")
  .map((r) => r.trim())
  .filter(Boolean);

const createArgs = [
  "pr",
  "create",
  "--repo",
  repo,
  "--base",
  "dev",
  "--head",
  branch,
  "--title",
  title,
  "--body-file",
  bodyFile,
];
for (const reviewer of reviewers) {
  createArgs.push("--reviewer", reviewer);
}
gh(createArgs);
console.log(`Opened PR for ${branch} covering issue(s): ${issuesForPr.join(", ")}`);
if (reviewers.length > 0) {
  console.log(`Requested review from: ${reviewers.join(", ")}`);
}
