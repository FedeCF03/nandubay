import { useMemo, useState } from 'react'
import {
  AlertTriangle, ArrowLeftRight, History, MoreHorizontal, PackagePlus, Pencil, Plus, Scale, Search,
} from 'lucide-react'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card } from '@/components/ui/card'
import {
  DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuSeparator, DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { Input } from '@/components/ui/input'
import { Skeleton } from '@/components/ui/skeleton'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { usePermiso } from '@/auth/sesion'
import { cn } from '@/lib/utils'
import {
  detalle, fmtNumero, fmtPesos, nombreTipo, unidadCorta, useDepositos, useProductos,
  type Producto, type ProductoConStock, type TipoOperacion,
} from '@/stock/api'
import { DialogoOperacion } from '@/stock/DialogoOperacion'
import { FormProducto } from '@/stock/FormProducto'
import { Movimientos } from '@/stock/Movimientos'
import { Retazos } from '@/stock/Retazos'

type Accion =
  | { tipo: 'producto'; producto: Partial<Producto> | null }
  | { tipo: 'operacion'; operacion: TipoOperacion; producto: ProductoConStock }
  | { tipo: 'movimientos'; producto: ProductoConStock }

export default function Stock() {
  const [tipo, setTipo] = useState('')
  const [buscar, setBuscar] = useState('')
  const [soloBajo, setSoloBajo] = useState(false)
  const [accion, setAccion] = useState<Accion | null>(null)
  const productos = useProductos({ tipo: tipo === 'retazos' ? 'placa' : tipo, buscar })
  const depositos = useDepositos().data ?? []
  const editaProductos = usePermiso('productos.editar')
  const editaStock = usePermiso('stock.editar')
  const veStock = usePermiso('stock.ver')

  const lista = useMemo(
    () => (productos.data ?? []).filter((p) => !soloBajo || p.bajo),
    [productos.data, soloBajo],
  )
  const bajos = (productos.data ?? []).filter((p) => p.bajo).length
  // Con pocos depósitos se muestra una columna por depósito.
  const columnas = depositos.length > 1 && depositos.length <= 4 ? depositos : []

  const acciones = (p: ProductoConStock) => (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button variant="ghost" size="icon" aria-label={`Acciones de ${p.nombre}`}><MoreHorizontal /></Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end" className="w-52">
        {editaStock && (
          <>
            <DropdownMenuItem onSelect={() => setAccion({ tipo: 'operacion', operacion: 'ingresos', producto: p })}>
              <PackagePlus /> Ingreso
            </DropdownMenuItem>
            <DropdownMenuItem onSelect={() => setAccion({ tipo: 'operacion', operacion: 'traspasos', producto: p })}>
              <ArrowLeftRight /> Traspaso
            </DropdownMenuItem>
            <DropdownMenuItem onSelect={() => setAccion({ tipo: 'operacion', operacion: 'ajustes', producto: p })}>
              <Scale /> Ajuste por conteo
            </DropdownMenuItem>
            <DropdownMenuSeparator />
          </>
        )}
        {veStock && (
          <DropdownMenuItem onSelect={() => setAccion({ tipo: 'movimientos', producto: p })}>
            <History /> Ver movimientos
          </DropdownMenuItem>
        )}
        {editaProductos && (
          <DropdownMenuItem onSelect={() => setAccion({ tipo: 'producto', producto: p })}>
            <Pencil /> Editar
          </DropdownMenuItem>
        )}
      </DropdownMenuContent>
    </DropdownMenu>
  )

  const cantidad = (p: ProductoConStock, n: number) => (
    <span className="tabular-nums">{fmtNumero(n)} <span className="text-xs text-muted-foreground">{unidadCorta[p.unidad]}</span></span>
  )

  return (
    <div className="space-y-5">
      <div className="flex flex-wrap items-end justify-between gap-3">
        <div>
          <h2 className="text-2xl font-semibold tracking-tight">Stock</h2>
          <p className="text-muted-foreground">Placas, cantos y herrajes en todos los depósitos.</p>
        </div>
        {editaProductos && (
          <Button onClick={() => setAccion({ tipo: 'producto', producto: null })}>
            <Plus /> Nuevo producto
          </Button>
        )}
      </div>

      <div className="flex flex-col gap-3 md:flex-row md:items-center">
        <div className="-mx-4 overflow-x-auto px-4 md:mx-0 md:px-0">
        <Tabs value={tipo || 'todos'} onValueChange={(v) => setTipo(v === 'todos' ? '' : v)}>
          <TabsList>
            <TabsTrigger value="todos">Todos</TabsTrigger>
            <TabsTrigger value="placa">Placas</TabsTrigger>
            <TabsTrigger value="canto">Cantos</TabsTrigger>
            <TabsTrigger value="herraje">Herrajes</TabsTrigger>
            <TabsTrigger value="otro">Otros</TabsTrigger>
            {veStock && <TabsTrigger value="retazos">Retazos</TabsTrigger>}
          </TabsList>
        </Tabs>
        </div>
        {tipo !== 'retazos' && <>
        <div className="relative flex-1">
          <Search className="pointer-events-none absolute top-1/2 left-2.5 size-4 -translate-y-1/2 text-muted-foreground" />
          <Input className="pl-8" placeholder="Buscar por nombre, código, color o marca" value={buscar}
            onChange={(e) => setBuscar(e.target.value)} />
        </div>
        {bajos > 0 && (
          <Button variant={soloBajo ? 'default' : 'outline'} onClick={() => setSoloBajo(!soloBajo)}
            className={cn(!soloBajo && 'border-amber-500/50 text-amber-700 dark:text-amber-400')}>
            <AlertTriangle /> {bajos} con stock bajo
          </Button>
        )}
        </>}
      </div>

      {tipo === 'retazos' ? <Retazos /> : productos.isPending ? (
        <div className="space-y-2">{Array.from({ length: 5 }, (_, i) => <Skeleton key={i} className="h-14" />)}</div>
      ) : lista.length === 0 ? (
        <div className="flex flex-col items-center gap-2 rounded-xl border border-dashed py-16 text-center">
          <PackagePlus className="size-10 text-muted-foreground" />
          <p className="font-medium">{buscar || tipo || soloBajo ? 'No hay productos que coincidan.' : 'Todavía no cargaste productos.'}</p>
          {editaProductos && !buscar && !tipo && (
            <Button variant="outline" onClick={() => setAccion({ tipo: 'producto', producto: null })}>
              <Plus /> Cargar el primero
            </Button>
          )}
        </div>
      ) : (
        <>
          {/* Compu: tabla */}
          <Card className="hidden py-0 md:block">
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead className="pl-4">Producto</TableHead>
                  {columnas.map((d) => (
                    <TableHead key={d.id} className="text-right">{d.sucursal}</TableHead>
                  ))}
                  <TableHead className="text-right">Total</TableHead>
                  <TableHead className="text-right">Mínimo</TableHead>
                  <TableHead className="text-right">Precio</TableHead>
                  <TableHead className="w-12" />
                </TableRow>
              </TableHeader>
              <TableBody>
                {lista.map((p) => (
                  <TableRow key={p.id}>
                    <TableCell className="pl-4">
                      <div className="flex items-center gap-2 font-medium">
                        {p.nombre}
                        {!tipo && <Badge variant="secondary" className="font-normal">{nombreTipo[p.tipo]}</Badge>}
                      </div>
                      <div className="text-xs text-muted-foreground">
                        {[p.sku, detalle(p)].filter(Boolean).join(' · ')}
                      </div>
                    </TableCell>
                    {columnas.map((d) => (
                      <TableCell key={d.id} className="text-right">{cantidad(p, p.por_deposito[d.id] ?? 0)}</TableCell>
                    ))}
                    <TableCell className="text-right font-semibold">
                      <span className={cn('inline-flex items-center gap-1', p.bajo && 'text-amber-700 dark:text-amber-400')}>
                        {p.bajo && <AlertTriangle className="size-3.5" />}{cantidad(p, p.total)}
                      </span>
                    </TableCell>
                    <TableCell className="text-right text-muted-foreground tabular-nums">
                      {p.stock_minimo ? fmtNumero(p.stock_minimo) : '–'}
                    </TableCell>
                    <TableCell className="text-right tabular-nums">{fmtPesos(p.precio_venta)}</TableCell>
                    <TableCell>{acciones(p)}</TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </Card>

          {/* Celular: tarjetas */}
          <div className="space-y-2 md:hidden">
            {lista.map((p) => (
              <Card key={p.id} className="gap-2 px-4 py-3">
                <div className="flex items-start justify-between gap-2">
                  <div className="min-w-0">
                    <div className="font-medium">{p.nombre}</div>
                    <div className="truncate text-xs text-muted-foreground">{[p.sku, detalle(p)].filter(Boolean).join(' · ')}</div>
                  </div>
                  {acciones(p)}
                </div>
                <div className="flex flex-wrap items-center gap-x-4 gap-y-1 text-sm">
                  <span className={cn('font-semibold', p.bajo && 'text-amber-700 dark:text-amber-400')}>
                    {p.bajo && <AlertTriangle className="mr-1 inline size-3.5" />}Total {cantidad(p, p.total)}
                  </span>
                  {depositos.length > 1 && depositos.map((d) => (
                    <span key={d.id} className="text-xs text-muted-foreground">
                      {d.sucursal}: {fmtNumero(p.por_deposito[d.id] ?? 0)}
                    </span>
                  ))}
                  <span className="ml-auto tabular-nums">{fmtPesos(p.precio_venta)}</span>
                </div>
              </Card>
            ))}
          </div>
        </>
      )}

      {accion?.tipo === 'producto' && <FormProducto producto={accion.producto} onClose={() => setAccion(null)} />}
      {accion?.tipo === 'operacion' && (
        <DialogoOperacion tipo={accion.operacion} producto={accion.producto} onClose={() => setAccion(null)} />
      )}
      {accion?.tipo === 'movimientos' && <Movimientos producto={accion.producto} onClose={() => setAccion(null)} />}
    </div>
  )
}
