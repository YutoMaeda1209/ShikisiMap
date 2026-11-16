import { createContext, useContext } from "react";
import type { Me } from "./api";

export type AuthContextType = {
  /** The signed-in user, or null when signed out or still loading. */
  me: Me | null;
  /** True until the first check of the session finishes. */
  isLoading: boolean;
  /** Leaves the page for the Google login. */
  login: () => void;
  logout: () => Promise<void>;
  updateMe: (patch: Partial<Pick<Me, "displayName" | "profilePublic">>) => Promise<void>;
  deleteAccount: () => Promise<void>;
};

export const AuthContext = createContext<AuthContextType | undefined>(undefined);

export function useAuth(): AuthContextType {
  const ctx = useContext(AuthContext);
  if (!ctx) throw new Error("useAuth must be used within an AuthProvider");
  return ctx;
}
