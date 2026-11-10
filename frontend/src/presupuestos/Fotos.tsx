import { useRef, useState } from 'react'
import { Camera, ChevronLeft, ChevronRight, Loader2, Trash2 } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Dialog, DialogContent, DialogDescription, DialogTitle } from '@/components/ui/dialog'
import { urlFoto, useFotos, type FotoMueble } from './api'

// FotosMueble muestra las fotos de un mueble del presupuesto y deja agregar
// más (en el celular abre la cámara o la galería) o borrarlas.
export function FotosMueble({ pedidoId, clave, nombre, fotos, edita }: {
  pedidoId: number
  clave: string
  nombre: string
  fotos: FotoMueble[]
  edita: boolean
}) {
  const entrada = useRef<HTMLInputElement>(null)
  const { subir, borrar } = useFotos(pedidoId)
  const [viendo, setViendo] = useState<number | null>(null)
  const actual = viendo == null ? undefined : fotos[viendo]
  const error = subir.error ?? borrar.error

  if (!edita && fotos.length === 0) return null
  return (
    <div className="space-y-2">
      <div className="text-sm font-medium">Fotos</div>
      <div className="flex flex-wrap gap-2">
        {fotos.map((f, i) => (
          <button key={f.id} type="button" onClick={() => setViendo(i)} aria-label={`Ver foto ${i + 1} de ${nombre}`}
            className="size-20 overflow-hidden rounded-md border bg-muted focus-visible:ring-2 focus-visible:ring-ring focus-visible:outline-none">
            <img src={urlFoto(pedidoId, f.id)} alt="" loading="lazy" className="size-full object-cover" />
          </button>
        ))}
        {edita && (
          <button type="button" onClick={() => entrada.current?.click()} disabled={subir.isPending}
            aria-label={`Agregar fotos a ${nombre}`}
            className="flex size-20 flex-col items-center justify-center gap-1 rounded-md border border-dashed text-xs text-muted-foreground hover:bg-accent disabled:opacity-60">
            {subir.isPending ? <Loader2 className="size-5 animate-spin" /> : <Camera className="size-5" />}
            {subir.isPending ? 'Subiendo…' : 'Agregar'}
          </button>
        )}
      </div>
      {error && <p className="text-sm text-destructive">{error.message}</p>}
      <input ref={entrada} type="file" accept="image/*" multiple hidden
        onChange={(e) => {
          const archivos = Array.from(e.target.files ?? [])
          e.target.value = ''
          if (archivos.length) subir.mutate({ clave, archivos })
        }} />

      <Dialog open={actual != null} onOpenChange={(o) => !o && setViendo(null)}>
        <DialogContent className="max-w-[min(64rem,calc(100%-2rem))] sm:max-w-[min(64rem,calc(100%-2rem))]">
          <DialogTitle>{nombre}</DialogTitle>
          <DialogDescription className="sr-only">Foto {(viendo ?? 0) + 1} de {fotos.length}</DialogDescription>
          {actual && (
            <img src={urlFoto(pedidoId, actual.id)} alt={`Foto ${(viendo ?? 0) + 1} de ${nombre}`}
              className="max-h-[70svh] w-full rounded-md object-contain" />
          )}
          <div className="flex items-center justify-between gap-2">
            <div className="flex items-center gap-1">
              <Button variant="outline" size="icon" aria-label="Foto anterior" disabled={!viendo}
                onClick={() => setViendo((v) => (v ? v - 1 : v))}><ChevronLeft /></Button>
              <span className="w-14 text-center text-sm tabular-nums text-muted-foreground">{(viendo ?? 0) + 1} / {fotos.length}</span>
              <Button variant="outline" size="icon" aria-label="Foto siguiente" disabled={viendo == null || viendo >= fotos.length - 1}
                onClick={() => setViendo((v) => (v != null && v < fotos.length - 1 ? v + 1 : v))}><ChevronRight /></Button>
            </div>
            {edita && actual && (
              <Button variant="ghost" className="text-destructive" disabled={borrar.isPending}
                onClick={() => borrar.mutate(actual.id, { onSuccess: () => setViendo(fotos.length > 1 ? Math.max(0, (viendo ?? 0) - 1) : null) })}>
                <Trash2 /> Borrar foto
              </Button>
            )}
          </div>
        </DialogContent>
      </Dialog>
    </div>
  )
}
