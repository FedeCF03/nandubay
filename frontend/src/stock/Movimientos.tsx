import { Sheet, SheetContent, SheetDescription, SheetHeader, SheetTitle } from '@/components/ui/sheet'
import { Skeleton } from '@/components/ui/skeleton'
import { cn } from '@/lib/utils'
import { fmtNumero, unidadCorta, useMovimientos, type Movimiento, type ProductoConStock } from './api'

const nombreMov: Record<Movimiento['tipo'], string> = {
  compra: 'Ingreso', venta: 'Venta', ajuste: 'Ajuste', traspaso_entrada: 'Traspaso (entra)',
  traspaso_salida: 'Traspaso (sale)', devolucion: 'Devolución', corte: 'Corte',
}

const fecha = new Intl.DateTimeFormat('es-AR', { dateStyle: 'short', timeStyle: 'short' })

export function Movimientos({ producto, onClose }: { producto: ProductoConStock; onClose: () => void }) {
  const { data, isPending } = useMovimientos(producto.id)
  const u = unidadCorta[producto.unidad] ?? ''
  return (
    <Sheet open onOpenChange={(o) => !o && onClose()}>
      <SheetContent className="w-full overflow-y-auto sm:max-w-md">
        <SheetHeader>
          <SheetTitle>Movimientos</SheetTitle>
          <SheetDescription>{producto.nombre}</SheetDescription>
        </SheetHeader>
        <div className="space-y-2 px-4 pb-6">
          {isPending && Array.from({ length: 4 }, (_, i) => <Skeleton key={i} className="h-16" />)}
          {data?.length === 0 && <p className="text-sm text-muted-foreground">Todavía no hay movimientos.</p>}
          {data?.map((m) => (
            <div key={m.id} className="rounded-lg border p-3 text-sm">
              <div className="flex items-baseline justify-between gap-2">
                <span className="font-medium">{nombreMov[m.tipo]}</span>
                <span className={cn('font-semibold tabular-nums', m.cantidad > 0 ? 'text-emerald-700 dark:text-emerald-400' : 'text-destructive')}>
                  {m.cantidad > 0 ? '+' : ''}{fmtNumero(m.cantidad)} {u}
                </span>
              </div>
              <div className="text-xs text-muted-foreground">
                {m.deposito} · {fecha.format(new Date(m.fecha))}{m.usuario ? ` · ${m.usuario}` : ''}
              </div>
              {m.nota && <div className="mt-1 text-xs">{m.nota}</div>}
            </div>
          ))}
        </div>
      </SheetContent>
    </Sheet>
  )
}
