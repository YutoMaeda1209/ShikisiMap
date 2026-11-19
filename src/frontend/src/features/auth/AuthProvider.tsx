import { useEffect, useState, type ReactNode } from "react";
import { api, ApiError, type Me } from "../../lib/api";
import { AuthContext } from "./authContext";

// Tracks who is signed in. The browser holds only the session cookie, which
// scripts cannot read, so the state comes from GET /api/me.
function AuthProvider({ children }: { children: ReactNode }) {
  const [me, setMe] = useState<Me | null>(null);
  const [isLoading, setIsLoading] = useState(true);

  useEffect(() => {
    let cancelled = false;
    api<Me>("/me")
      .then((user) => {
        if (!cancelled) setMe(user);
      })
      .catch((err: unknown) => {
        // 401 is the normal answer for a visitor who is not signed in.
        if (!(err instanceof ApiError && err.status === 401)) {
          console.warn("Could not check the login state:", err);
        }
      })
      .finally(() => {
        if (!cancelled) setIsLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, []);

  const login = () => {
    window.location.assign("/api/auth/google/login");
  };

  const logout = async () => {
    await api<void>("/session", { method: "DELETE" });
    setMe(null);
  };

  const updateMe = async (patch: Partial<Pick<Me, "displayName" | "profilePublic">>) => {
    setMe(await api<Me>("/me", { method: "PATCH", body: patch }));
  };

  const deleteAccount = async () => {
    await api<void>("/me", { method: "DELETE" });
    setMe(null);
  };

  return (
    <AuthContext.Provider value={{ me, isLoading, login, logout, updateMe, deleteAccount }}>
      {children}
    </AuthContext.Provider>
  );
}

export default AuthProvider;
