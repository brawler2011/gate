'use client'

import { useEffect } from 'react'

export default function ReactScan() {
  useEffect(() => {
    const initScan = async () => {
      const { scan } = await import('react-scan')
      scan({
        enabled: true,
        log: true,
      })
    }
    
    initScan()
  }, [])

  return null
}