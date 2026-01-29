import { createContext, useContext } from 'react';

type LocationSelectionContextType = {
  selectedId: string | null
  select: (id: string | null) => void
};

export const LocationSelectionContext = createContext<LocationSelectionContextType | undefined>(undefined);

export function useLocationSelection(): LocationSelectionContextType {
  const ctx = useContext(LocationSelectionContext);
  if (!ctx) throw new Error('useLocationSelection must be used within Provider');
  return ctx;
}
