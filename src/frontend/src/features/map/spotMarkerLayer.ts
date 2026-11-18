// The spot pins and their clusters on a Leaflet map. This is plain imperative
// code: the cluster group owns the placement of the markers, so they cannot be
// React elements.
import L from "leaflet";
// Extends the global L that leaflet sets, so it must come after "leaflet".
import "leaflet.markercluster";
import "leaflet.markercluster/dist/MarkerCluster.css";
import type { Spot } from "../../lib/api";
import {
  clusterSize,
  createClusterElement,
  createPinElement,
  spotState,
  updatePinElement,
  type SpotState,
} from "./markerElements";

type Entry = {
  spot: Spot;
  marker: L.Marker;
  element: HTMLDivElement;
};

// From this zoom level on, every spot is drawn as its own pin.
const noClusterZoom = 15;

// The pin is 28 x 36 px and points at its position with its bottom centre.
const pinSize: L.PointTuple = [28, 36];
const pinAnchor: L.PointTuple = [14, 36];

// Leaflet orders markers by their latitude; the offsets put the closed spots
// below the others and the selected one above everything.
const zIndexByState: Record<SpotState, number> = { closed: -10000, unvisited: 0, visited: 1000 };
const selectedZIndex = 100000;

export class SpotMarkerLayer {
  private readonly map: L.Map;
  private readonly onSelect: (spotId: string) => void;
  private readonly group: L.MarkerClusterGroup;
  private readonly entries = new Map<string, Entry>();
  private readonly spotByMarker = new WeakMap<L.Marker, Spot>();
  private visitedSpotIds: ReadonlySet<string> = new Set();
  // The selected spot is drawn by a marker of its own, outside the cluster
  // group, so that it stays visible at every zoom level. Its pin in the group
  // is hidden meanwhile but stays there, so that the clusters do not change.
  private selected: Entry | null = null;

  constructor(map: L.Map, onSelect: (spotId: string) => void) {
    this.map = map;
    this.onSelect = onSelect;
    this.group = L.markerClusterGroup({
      // maxClusterRadius is in pixels, like the radius of the clusters.
      maxClusterRadius: 60,
      disableClusteringAtZoom: noClusterZoom,
      showCoverageOnHover: false,
      iconCreateFunction: (cluster) => {
        // Closed spots count for neither side of the visited share.
        let open = 0;
        let visited = 0;
        for (const marker of cluster.getAllChildMarkers()) {
          const spot = this.spotByMarker.get(marker);
          if (!spot || spot.isClosed) continue;
          open++;
          if (this.visitedSpotIds.has(spot.id)) visited++;
        }
        const count = cluster.getChildCount();
        const size = clusterSize(count);
        return L.divIcon({
          html: createClusterElement(count, open === 0 ? 0 : visited / open),
          // An empty class name drops Leaflet's default white box.
          className: "",
          iconSize: [size, size],
        });
      },
    });
    map.addLayer(this.group);
  }

  private createMarker(spot: Spot, zIndexOffset = 0): Entry {
    const element = createPinElement();
    const marker = L.marker([spot.location.lat, spot.location.lng], {
      icon: L.divIcon({ html: element, className: "", iconSize: pinSize, iconAnchor: pinAnchor }),
      // Shown as a tooltip on hover.
      title: spot.name,
      alt: spot.name,
      zIndexOffset,
    });
    marker.on("click", () => this.onSelect(spot.id));
    return { spot, marker, element };
  }

  /**
   * Makes the markers match the spots. Markers of spots that stay are kept,
   * so that recording a visit or typing in the search does not redraw them
   * all. Call update() afterwards to draw the result.
   */
  setSpots(spots: Spot[]): void {
    const shownIds = new Set(spots.map((s) => s.id));
    const removed: L.Marker[] = [];
    const added: L.Marker[] = [];

    for (const [id, entry] of this.entries) {
      if (shownIds.has(id)) continue;
      removed.push(entry.marker);
      this.entries.delete(id);
    }

    for (const spot of spots) {
      const existing = this.entries.get(spot.id);
      if (existing) {
        if (existing.spot === spot) continue;
        // The spot object is replaced when the spots are loaded again.
        const changed =
          existing.spot.location.lat !== spot.location.lat ||
          existing.spot.location.lng !== spot.location.lng ||
          existing.spot.name !== spot.name;
        if (!changed) {
          existing.spot = spot;
          this.spotByMarker.set(existing.marker, spot);
          continue;
        }
        removed.push(existing.marker);
      }
      const entry = this.createMarker(spot);
      this.entries.set(spot.id, entry);
      this.spotByMarker.set(entry.marker, spot);
      added.push(entry.marker);
    }

    // The batch methods cluster once instead of once per marker.
    if (removed.length > 0) this.group.removeLayers(removed);
    if (added.length > 0) this.group.addLayers(added);
  }

  /** Shows the state of every marker and redraws the clusters. */
  update(visitedSpotIds: ReadonlySet<string>, selectedId: string | null): void {
    this.visitedSpotIds = visitedSpotIds;

    for (const { spot, marker, element } of this.entries.values()) {
      const state = spotState(spot, visitedSpotIds);
      updatePinElement(element, state, false);
      // Covered by the selected marker; hidden so that the two do not show at once.
      element.classList.toggle("hiddenBySelection", spot.id === selectedId);
      marker.setZIndexOffset(zIndexByState[state]);
    }

    // A selected spot that is filtered out of the map is not shown either.
    const entry = selectedId ? this.entries.get(selectedId) : undefined;
    if (this.selected && this.selected.spot !== entry?.spot) {
      this.selected.marker.remove();
      this.selected = null;
    }
    if (entry && !this.selected) {
      this.selected = this.createMarker(entry.spot, selectedZIndex);
      this.selected.marker.addTo(this.map);
    }
    if (this.selected) {
      updatePinElement(this.selected.element, spotState(this.selected.spot, visitedSpotIds), true);
    }

    // The visited share of the clusters may have changed.
    this.group.refreshClusters();
  }

  destroy(): void {
    this.map.removeLayer(this.group);
    this.group.clearLayers();
    this.entries.clear();
    this.selected?.marker.remove();
    this.selected = null;
  }
}
