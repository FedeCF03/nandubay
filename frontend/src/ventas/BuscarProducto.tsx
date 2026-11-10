import { useState } from 'react'
import { Search } from 'lucide-react'
import {
  Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle,
} from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Skeleton } from '@/components/ui/skeleton'
import { detalle, fmtNumero, fmtPesos, unidadCorta, useProductos, type ProductoConStock } from '@/stock/api'

export function BuscarProducto({ onElegir, onClose, sinPlacas, titulo = 'Agregar producto', descripcion = 'Placas, cantos o herrajes del stock.' }: {
  onElegir: (p: ProductoConStock) => void
  onClose: () => void
  sinPlacas?: boolean
  titulo?: string
  descripcion?: string
}) {
  const [buscar, setBuscar] = useState('')
  const productos = useProductos({ tipo: '', buscar })
  const lista = (productos.data ?? []).filter((p) => !sinPlacas || p.tipo !== 'placa')
  return (
    <Dialog open onOpenChange={(o) => !o && onClose()}>
      <DialogContent className="max-h-[85svh] grid-rows-[auto_auto_1fr]">
        <DialogHeader>
          <DialogTitle>{titulo}</DialogTitle>
          <DialogDescription>{descripcion}</DialogDescription>
        </DialogHeader>
        <div className="relative">
          <Search className="pointer-events-none absolute top-1/2 left-2.5 size-4 -translate-y-1/2 text-muted-foreground" />
          <Input className="pl-8" autoFocus placeholder="Nombre, código o color" value={buscar}
            onChange={(e) => setBuscar(e.target.value)} aria-label="Buscar producto" />
        </div>
        <div className="-mx-2 min-h-0 overflow-y-auto">
          {productos.isPending ? (
            <div className="space-y-2 px-2">{Array.from({ length: 4 }, (_, i) => <Skeleton key={i} className="h-12" />)}</div>
          ) : lista.length === 0 ? (
            <p className="px-2 py-6 text-center text-sm text-muted-foreground">No hay productos que coincidan.</p>
          ) : (
            lista.map((p) => (
              <button key={p.id} type="button" onClick={() => onElegir(p)}
                className="flex w-full items-center justify-between gap-3 rounded-md px-2 py-2 text-left hover:bg-accent">
                <div className="min-w-0">
                  <div className="truncate text-sm font-medium">{p.nombre}</div>
                  <div className="truncate text-xs text-muted-foreground">{detalle(p)}</div>
                </div>
                <div className="shrink-0 text-right text-sm">
                  <div className="tabular-nums">{fmtPesos(p.precio_venta)}</div>
                  <div className="text-xs text-muted-foreground">{fmtNumero(p.total)} {unidadCorta[p.unidad]} en stock</div>
                </div>
              </button>
            ))
          )}
        </div>
      </DialogContent>
    </Dialog>
  )
}
