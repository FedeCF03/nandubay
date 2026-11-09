import { Hammer } from 'lucide-react'
import { useLocation } from 'wouter'
import { tituloDe } from '@/layout/navegacion'

export default function Proximamente() {
  const [pathname] = useLocation()
  return (
    <div className="flex flex-col items-center justify-center rounded-xl border border-dashed py-24 text-center">
      <Hammer className="mb-3 size-10 text-muted-foreground" />
      <h2 className="text-lg font-semibold">{tituloDe(pathname)}</h2>
      <p className="text-sm text-muted-foreground">Esta pantalla todavía está en el taller.</p>
    </div>
  )
}
