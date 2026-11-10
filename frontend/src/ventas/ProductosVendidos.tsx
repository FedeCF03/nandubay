import { useState } from 'react'
import { BarChart3 } from 'lucide-react'
import { Card } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Skeleton } from '@/components/ui/skeleton'
import { Table, TableBody, TableCell, TableFooter, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import { useUsuario } from '@/auth/sesion'
import { fmtNumero, fmtPesos, unidadCorta } from '@/stock/api'
import { fechaISO, useProductosVendidos } from './api'

// ProductosVendidos resume cuánto se vendió de cada producto en un período.
export function ProductosVendidos() {
  const u = useUsuario()
  const hoy = new Date()
  const [desde, setDesde] = useState(fechaISO(new Date(hoy.getFullYear(), hoy.getMonth(), 1)))
  const [hasta, setHasta] = useState(fechaISO(hoy))
  const [sucursal, setSucursal] = useState('todas')
  const vendidos = useProductosVendidos(desde, hasta, sucursal === 'todas' ? '' : sucursal)
  const lista = vendidos.data ?? []
  const importe = lista.reduce((t, v) => t + v.importe, 0)
  // Sin costo cargado no se puede saber la ganancia de ese producto.
  const conCosto = lista.filter((v) => v.costo > 0)
  const ganancia = conCosto.reduce((t, v) => t + v.importe - v.costo, 0)
  const sinCosto = lista.length - conCosto.length

  return (
    <div className="space-y-4">
      <div className="grid grid-cols-2 gap-3 sm:flex sm:items-end">
        <label className="grid gap-1.5 text-sm font-medium">
          Desde
          <Input type="date" value={desde} max={hasta} onChange={(e) => setDesde(e.target.value)} />
        </label>
        <label className="grid gap-1.5 text-sm font-medium">
          Hasta
          <Input type="date" value={hasta} min={desde} onChange={(e) => setHasta(e.target.value)} />
        </label>
        {u.sucursales.length > 1 && (
          <div className="col-span-2 grid gap-1.5 text-sm font-medium sm:w-56">
            <span>Sucursal</span>
            <Select value={sucursal} onValueChange={setSucursal}>
              <SelectTrigger className="w-full" aria-label="Sucursal"><SelectValue /></SelectTrigger>
              <SelectContent>
                <SelectItem value="todas">Todas</SelectItem>
                {u.sucursales.map((s) => <SelectItem key={s.id} value={String(s.id)}>{s.nombre}</SelectItem>)}
              </SelectContent>
            </Select>
          </div>
        )}
      </div>

      {vendidos.isError ? (
        <p className="text-sm text-destructive">{vendidos.error.message}</p>
      ) : vendidos.isPending ? (
        <div className="space-y-2">{Array.from({ length: 4 }, (_, i) => <Skeleton key={i} className="h-12" />)}</div>
      ) : lista.length === 0 ? (
        <div className="flex flex-col items-center gap-2 rounded-xl border border-dashed py-16 text-center">
          <BarChart3 className="size-10 text-muted-foreground" />
          <p className="font-medium">No hubo ventas confirmadas en ese período.</p>
        </div>
      ) : (
        <>
          <div className="grid grid-cols-2 gap-3 sm:max-w-md">
            <Card className="gap-1 px-4 py-3">
              <span className="text-xs text-muted-foreground">Vendido</span>
              <span className="text-lg font-semibold tabular-nums">{fmtPesos(importe)}</span>
            </Card>
            <Card className="gap-1 px-4 py-3">
              <span className="text-xs text-muted-foreground">Ganancia sobre costo</span>
              <span className="text-lg font-semibold tabular-nums">{conCosto.length ? fmtPesos(ganancia) : '—'}</span>
              {sinCosto > 0 && (
                <span className="text-xs text-muted-foreground">
                  {sinCosto === 1 ? 'Un producto no tiene' : `${sinCosto} productos no tienen`} costo cargado
                </span>
              )}
            </Card>
          </div>
          <Card className="py-0">
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead className="pl-4">Producto</TableHead>
                  <TableHead className="text-right">Cantidad</TableHead>
                  <TableHead className="hidden text-right sm:table-cell">Ventas</TableHead>
                  <TableHead className="hidden text-right sm:table-cell">Ganancia</TableHead>
                  <TableHead className="pr-4 text-right">Importe</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {lista.map((v) => (
                  <TableRow key={v.producto_id}>
                    <TableCell className="pl-4 font-medium whitespace-normal">{v.nombre}</TableCell>
                    <TableCell className="text-right tabular-nums">
                      {fmtNumero(v.cantidad)} <span className="text-xs text-muted-foreground">{unidadCorta[v.unidad]}</span>
                    </TableCell>
                    <TableCell className="hidden text-right tabular-nums sm:table-cell">{v.ventas}</TableCell>
                    <TableCell className="hidden text-right tabular-nums sm:table-cell">
                      {v.costo > 0 ? fmtPesos(v.importe - v.costo) : <span className="text-muted-foreground">—</span>}
                    </TableCell>
                    <TableCell className="pr-4 text-right tabular-nums">{fmtPesos(v.importe)}</TableCell>
                  </TableRow>
                ))}
              </TableBody>
              <TableFooter>
                <TableRow>
                  <TableCell className="pl-4">Total</TableCell>
                  <TableCell />
                  <TableCell className="hidden sm:table-cell" />
                  <TableCell className="hidden text-right tabular-nums sm:table-cell">{conCosto.length ? fmtPesos(ganancia) : '—'}</TableCell>
                  <TableCell className="pr-4 text-right tabular-nums">{fmtPesos(importe)}</TableCell>
                </TableRow>
              </TableFooter>
            </Table>
          </Card>
        </>
      )}
    </div>
  )
}
