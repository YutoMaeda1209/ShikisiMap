// The spot pins and their clusters on a Google map. This is plain imperative
// code: the clusterer owns the placement of the markers, so they cannot be
// React elements.
import { MarkerClusterer, SuperClusterAlgorithm } from "@googlemaps/markerclusterer";
import type { Spot } from "./api";
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
  // The selected pin is taken out of the clusterer so that it stays visible
  // at every zoom level.
  private standaloneId: string | null = null;

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

  /**
   * Makes the markers match the spots. Markers of spots that stay are kept,
   * so that recording a visit or typing in the search does not redraw them
   * all. Call update() afterwards to draw the result.
   */
  setSpots(spots: Spot[]): void {
    const shownIds = new Set(spots.map((s) => s.id));
    for (const [id, entry] of this.entries) {
      if (shownIds.has(id)) continue;
      if (this.standaloneId === id) {
        entry.marker.map = null;
        this.standaloneId = null;
      } else {
        this.clusterer.removeMarker(entry.marker, true);
      }
      this.entries.delete(id);
    }

    for (const spot of spots) {
      const existing = this.entries.get(spot.id);
      if (existing) {
        // The spot object is replaced when the spots are loaded again.
        const previous = existing.spot;
        if (previous === spot) continue;
        existing.spot = spot;
        this.spotByMarker.set(existing.marker, spot);
        if (previous.location.lat !== spot.location.lat || previous.location.lng !== spot.location.lng) {
          existing.marker.position = spot.location;
        }
        if (previous.name !== spot.name) {
          existing.marker.title = spot.name;
        }
        continue;
      }
      const element = createPinElement();
      const marker = new this.markerLibrary.AdvancedMarkerElement({
        position: spot.location,
        content: element,
        // Shown as a tooltip on hover.
        title: spot.name,
        gmpClickable: true,
      });
      marker.addEventListener("gmp-click", () => this.onSelect(spot.id));
      this.entries.set(spot.id, { spot, marker, element });
      this.spotByMarker.set(marker, spot);
      this.clusterer.addMarker(marker, true);
    }
  }

  /** Shows the state of every marker and redraws the clusters. */
  update(visitedSpotIds: ReadonlySet<string>, selectedId: string | null): void {
    this.visitedSpotIds = visitedSpotIds;

    for (const { spot, marker, element } of this.entries.values()) {
      const state = spotState(spot, visitedSpotIds);
      const selected = spot.id === selectedId;
      updatePinElement(element, state, selected);
      marker.zIndex = selected ? selectedZIndex : zIndexByState[state];
    }

    const previous = this.standaloneId ? this.entries.get(this.standaloneId) : undefined;
    const next = selectedId ? this.entries.get(selectedId) : undefined;
    if (previous && previous !== next) {
      previous.marker.map = null;
      this.clusterer.addMarker(previous.marker, true);
    }
    if (next && previous !== next) {
      this.clusterer.removeMarker(next.marker, true);
      next.marker.map = this.map;
    }
    this.standaloneId = next ? next.spot.id : null;

    // The members of the clusters or their visited share may have changed.
    this.clusterer.render();
  }

  destroy(): void {
    this.clusterer.clearMarkers();
    this.clusterer.setMap(null);
    for (const { marker } of this.entries.values()) {
      marker.map = null;
    }
    this.entries.clear();
    this.standaloneId = null;
  }
}
