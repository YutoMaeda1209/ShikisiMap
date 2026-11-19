import { parseRoute } from "./lib/route";
import MapPage from "./pages/MapPage";
import ProfilePage from "./pages/ProfilePage";
import "./App.css";

function App() {
  const route = parseRoute(window.location.pathname);
  return (
    <div id="app">
      {route.page === "profile" ? <ProfilePage userId={route.userId} /> : <MapPage />}
    </div>
  );
}

export default App;
