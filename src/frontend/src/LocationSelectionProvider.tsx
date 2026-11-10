import { useState } from 'react'
import { LocationSelectionContext } from './locationSelectionContext'

// Provider component to manage location selection state
function LocationSelectionProvider({ children }: { children: React.ReactNode }) {
  const [selectedId, setSelectedId] = useState<string | null>(null)

  const select = (id: string | null) => {
    setSelectedId(id)
    window.dispatchEvent(new CustomEvent("location:selected", { detail: id }))
  }

  return (
    <LocationSelectionContext.Provider value={{ selectedId, select }}>
      {children}
    </LocationSelectionContext.Provider>
  )
}

export default LocationSelectionProvider
