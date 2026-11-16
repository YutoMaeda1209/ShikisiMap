// DOM elements for the markers. AdvancedMarkerElement draws whatever element
// it is given, so the look lives in Map.css.
import type { Spot } from "./api";

export type SpotState = "unvisited" | "visited" | "closed";

export function spotState(spot: Spot, visitedSpotIds: ReadonlySet<string>): SpotState {
  // A closed spot looks closed whether or not it was visited.
  if (spot.isClosed) return "closed";
  return visitedSpotIds.has(spot.id) ? "visited" : "unvisited";
}

const svgNamespace = "http://www.w3.org/2000/svg";

/** A teardrop pin, 28 x 36 px, with an icon for the state in its centre. */
export function createPinElement(): HTMLDivElement {
  const root = document.createElement("div");

  const ripple = document.createElement("span");
  ripple.className = "spotPinRipple";

  const svg = document.createElementNS(svgNamespace, "svg");
  svg.setAttribute("viewBox", "0 0 28 36");
  svg.setAttribute("width", "28");
  svg.setAttribute("height", "36");
  svg.setAttribute("aria-hidden", "true");
  const path = document.createElementNS(svgNamespace, "path");
  path.setAttribute("d", "M14 1C6.82 1 1 6.82 1 14c0 9.5 13 21 13 21s13-11.5 13-21C27 6.82 21.18 1 14 1z");
  path.setAttribute("fill", "currentColor");
  path.setAttribute("stroke", "#fff");
  path.setAttribute("stroke-width", "2");
  svg.appendChild(path);

  // The glyph comes from CSS, chosen by the state class on the root.
  const icon = document.createElement("span");
  icon.className = "spotPinIcon";

  root.append(ripple, svg, icon);
  return root;
}

export function updatePinElement(root: HTMLElement, state: SpotState, selected: boolean): void {
  root.className = `spotPin ${state}${selected ? " selected" : ""}`;
}

/**
 * A white disc with the number of spots, ringed in green by the share of
 * them that are visited.
 */
export function createClusterElement(count: number, visitedRatio: number): HTMLDivElement {
  const root = document.createElement("div");
  const size = count < 10 ? "small" : count < 50 ? "medium" : "large";
  root.className = `spotCluster ${size}`;
  root.style.setProperty("--visited", `${Math.round(visitedRatio * 100)}%`);

  const label = document.createElement("span");
  label.className = "spotClusterCount";
  label.textContent = String(count);
  root.appendChild(label);
  return root;
}

export function createCurrentLocationElement(): HTMLDivElement {
  const root = document.createElement("div");
  root.className = "currentLocation";
  return root;
}
