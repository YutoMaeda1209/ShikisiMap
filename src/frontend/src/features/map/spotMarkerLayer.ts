// The spot pins and their clusters on a Google map. This is plain imperative
// code: the clusterer owns the placement of the markers, so they cannot be
// React elements.
import { MarkerClusterer, SuperClusterAlgorithm } from "@googlemaps/markerclusterer";
import type { Spot } from "../../lib/api";
import {
  createClusterElement,
  createPinElement,
  spotState,
  updatePinElement,
  type SpotState,
} from "./markerElements";

type AdvancedMarker = google.maps.marker.AdvancedMarkerElement;

type Entry = {
  spot: Spot;
  marker: AdvancedMarker;
  element: HTMLDivElement;
};

// From this zoom level on, every spot is drawn as its own pin.
const noClusterZoom = 15;

// Closed spots go below the others, the selected one above everything.
const zIndexByState: Record<SpotState, number> = { closed: 0, unvisited: 1, visited: 2 };
const selectedZIndex = 1000;
const clusterZIndex = 100;

export class SpotMarkerLayer {
  private readonly map: google.maps.Map;
  private readonly markerLibrary: google.maps.MarkerLibrary;
  private readonly onSelect: (spotId: string) => void;
  private readonly clusterer: MarkerClusterer;
  private readonly entries = new Map<string, Entry>();
  private readonly spotByMarker = new WeakMap<object, Spot>();
  private visitedSpotIds: ReadonlySet<string> = new Set();
  // The selected spot is drawn by a marker of its own, outside the clusterer,
  // so that it stays visible at every zoom level. Its pin in the clusterer is
  // hidden meanwhile but stays there: taking it out would make the clusterer
  // take the marker off the map on its next redraw, and would split clusters.
  private selected: Entry | null = null;

  constructor(
    map: google.maps.Map,
    markerLibrary: google.maps.MarkerLibrary,
    onSelect: (spotId: string) => void
  ) {
    this.map = map;
    this.markerLibrary = markerLibrary;
    this.onSelect = onSelect;
    this.clusterer = new MarkerClusterer({
      map,
      // maxZoom is the last zoom level that still clusters.
      algorithm: new SuperClusterAlgorithm({ maxZoom: noClusterZoom - 1, radius: 60 }),
      renderer: {
        render: (cluster) => {
          // Closed spots count for neither side of the visited share.
          let open = 0;
          let visited = 0;
          for (const marker of cluster.markers) {
            const spot = this.spotByMarker.get(marker);
            if (!spot || spot.isClosed) continue;
            open++;
            if (this.visitedSpotIds.has(spot.id)) visited++;
          }
          return new markerLibrary.AdvancedMarkerElement({
            position: cluster.position,
            content: createClusterElement(cluster.count, open === 0 ? 0 : visited / open),
            title: `${cluster.count} 件のスポット`,
            zIndex: clusterZIndex,
          });
        },
      },
    });
  }

  // The position and the title are only given when a marker is created:
  // a marker is replaced rather than changed, which keeps to the most common
  // use of AdvancedMarkerElement.
  private createMarker(spot: Spot, options: { map?: google.maps.Map; zIndex?: number } = {}): Entry {
    const element = createPinElement();
    const marker = new this.markerLibrary.AdvancedMarkerElement({
      ...options,
      position: spot.location,
      content: element,
      // Shown as a tooltip on hover.
      title: spot.name,
      gmpClickable: true,
    });
    marker.addEventListener("gmp-click", () => this.onSelect(spot.id));
    return { spot, marker, element };
  }

  /**
   * Makes the markers match the spots. Markers of spots that stay are kept,
   * so that recording a visit or typing in the search does not redraw them
   * all. Call update() afterwards to draw the result.
   */
  setSpots(spots: Spot[]): void {
    const shownIds = new Set(spots.map((s) => s.id));
    for (const [id, entry] of this.entries) {
      if (shownIds.has(id)) continue;
      this.clusterer.removeMarker(entry.marker, true);
      this.entries.delete(id);
    }

    for (const spot of spots) {
      const existing = this.entries.get(spot.id);
      if (existing) {
        if (existing.spot === spot) continue;
        // The spot object is replaced when the spots are loaded again.
        const moved =
          existing.spot.location.lat !== spot.location.lat ||
          existing.spot.location.lng !== spot.location.lng ||
          existing.spot.name !== spot.name;
        if (!moved) {
          existing.spot = spot;
          this.spotByMarker.set(existing.marker, spot);
          continue;
        }
        this.clusterer.removeMarker(existing.marker, true);
      }
      const entry = this.createMarker(spot);
      this.entries.set(spot.id, entry);
      this.spotByMarker.set(entry.marker, spot);
      this.clusterer.addMarker(entry.marker, true);
    }
  }

  /** Shows the state of every marker and redraws the clusters. */
  update(visitedSpotIds: ReadonlySet<string>, selectedId: string | null): void {
    this.visitedSpotIds = visitedSpotIds;

    for (const { spot, marker, element } of this.entries.values()) {
      const state = spotState(spot, visitedSpotIds);
      updatePinElement(element, state, false);
      // Covered by the selected marker; hidden so that the two do not show at once.
      element.classList.toggle("hiddenBySelection", spot.id === selectedId);
      marker.zIndex = zIndexByState[state];
    }

    // A selected spot that is filtered out of the map is not shown either.
    const entry = selectedId ? this.entries.get(selectedId) : undefined;
    if (this.selected && this.selected.spot !== entry?.spot) {
      this.selected.marker.map = null;
      this.selected = null;
    }
    if (entry && !this.selected) {
      this.selected = this.createMarker(entry.spot, { map: this.map, zIndex: selectedZIndex });
    }
    if (this.selected) {
      updatePinElement(this.selected.element, spotState(this.selected.spot, visitedSpotIds), true);
    }

    // The visited share of the clusters may have changed.
    this.clusterer.render();
  }

  destroy(): void {
    this.clusterer.clearMarkers();
    this.clusterer.setMap(null);
    for (const { marker } of this.entries.values()) {
      marker.map = null;
    }
    this.entries.clear();
    if (this.selected) this.selected.marker.map = null;
    this.selected = null;
  }
}
