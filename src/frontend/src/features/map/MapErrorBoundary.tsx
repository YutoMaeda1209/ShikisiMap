import { Component, type ErrorInfo, type ReactNode } from "react";

type Props = { children: ReactNode };
type State = { failed: boolean };

// The map depends on a third-party script that can fail on its own (network,
// API key, quota). Catching its errors here keeps the list and the visit
// records usable without the map.
class MapErrorBoundary extends Component<Props, State> {
  state: State = { failed: false };

  static getDerivedStateFromError(): State {
    return { failed: true };
  }

  componentDidCatch(error: Error, info: ErrorInfo) {
    console.error("The map failed:", error, info.componentStack);
  }

  render() {
    if (this.state.failed) {
      return (
        <div className="mapFallback">
          地図を表示できませんでした。ページを再読み込みしてください。
        </div>
      );
    }
    return this.props.children;
  }
}

export default MapErrorBoundary;
