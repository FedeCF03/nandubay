import { useState, type FormEvent } from 'react'
import { Loader2, MapPin } from 'lucide-react'
import { Button } from '@/components/ui/button'
import {
  Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle,
} from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { fmtNumero, unidadCorta, useDepositos, useGuardarUbicaciones, type ProductoConStock } from './api'

// DialogoUbicacion carga dónde está guardado el producto en cada depósito
// (estante, pasillo, rack…). Se muestra en el stock y al entregar una venta.
export function DialogoUbicacion({ producto, onClose }: { producto: ProductoConStock; onClose: () => void }) {
  const depositos = useDepositos().data ?? []
  const guardar = useGuardarUbicaciones()
  const [valores, setValores] = useState<Record<string, string>>(() => ({ ...producto.ubicaciones }))
  const u = unidadCorta[producto.unidad] ?? producto.unidad

  const enviar = (e: FormEvent) => {
    e.preventDefault()
    guardar.mutate({
      productoID: producto.id,
      ubicaciones: depositos.map((d) => ({ deposito_id: d.id, ubicacion: valores[String(d.id)] ?? '' })),
    }, { onSuccess: onClose })
  }

  return (
    <Dialog open onOpenChange={(o) => !o && onClose()}>
      <DialogContent className="sm:max-w-md">
        <form onSubmit={enviar} className="grid min-w-0 gap-4">
          <DialogHeader>
            <DialogTitle>Ubicación en el depósito</DialogTitle>
            <DialogDescription>
              {producto.nombre}. Dónde está guardado en cada depósito, para encontrarlo al entregar.
            </DialogDescription>
          </DialogHeader>
          <div className="grid gap-3">
            {depositos.map((d) => {
              const k = String(d.id)
              return (
                <div key={d.id} className="grid gap-1.5">
                  <Label htmlFor={`ub-${k}`} className="flex justify-between gap-2 text-xs text-muted-foreground">
                    <span>{d.sucursal} · {d.nombre}</span>
                    <span className="tabular-nums">{fmtNumero(producto.por_deposito[k] ?? 0)} {u}</span>
                  </Label>
                  <div className="relative">
                    <MapPin className="pointer-events-none absolute top-1/2 left-2.5 size-4 -translate-y-1/2 text-muted-foreground" />
                    <Input id={`ub-${k}`} className="pl-8" maxLength={60} placeholder="Ej: Estante B3, pasillo 2"
                      value={valores[k] ?? ''} onChange={(e) => setValores({ ...valores, [k]: e.target.value })} />
                  </div>
                </div>
              )
            })}
          </div>
          {guardar.isError && <p role="alert" className="text-sm text-destructive">{guardar.error.message}</p>}
          <DialogFooter>
            <Button type="button" variant="outline" onClick={onClose}>Cancelar</Button>
            <Button type="submit" disabled={guardar.isPending || depositos.length === 0}>
              {guardar.isPending && <Loader2 className="animate-spin" />} Guardar
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}
