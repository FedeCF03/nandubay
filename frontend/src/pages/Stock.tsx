import { useMemo, useState } from 'react'
import { useSearch } from 'wouter'
import {
  AlertTriangle, ArrowLeftRight, History, MapPin, MoreHorizontal, PackagePlus, Pencil, Plus, Scale, Search,
} from 'lucide-react'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card } from '@/components/ui/card'
import {
  DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuSeparator, DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { Input } from '@/components/ui/input'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Skeleton } from '@/components/ui/skeleton'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { usePermiso } from '@/auth/sesion'
import { cn } from '@/lib/utils'
import {
  detalle, fmtNumero, fmtPesos, nombreTipo, unidadCorta, useCategorias, useDepositos, useProductos,
  type Producto, type ProductoConStock, type TipoOperacion,
} from '@/stock/api'
import { DialogoOperacion } from '@/stock/DialogoOperacion'
import { DialogoUbicacion } from '@/stock/DialogoUbicacion'
import { FormProducto } from '@/stock/FormProducto'
import { Movimientos } from '@/stock/Movimientos'
import { Retazos } from '@/stock/Retazos'

type Accion =
  | { tipo: 'producto'; producto: Partial<Producto> | null }
  | { tipo: 'operacion'; operacion: TipoOperacion; producto: ProductoConStock }
  | { tipo: 'movimientos'; producto: ProductoConStock }
  | { tipo: 'ubicacion'; producto: ProductoConStock }

export default function Stock() {
  const [tipo, setTipo] = useState('')
  const [buscar, setBuscar] = useState('')
  // Desde la pantalla de categorías se llega con ?categoria=<id>.
  const search = useSearch()
  const [categoria, setCategoria] = useState(() => new URLSearchParams(search).get('categoria') ?? '')
  const categorias = useCategorias().data ?? []
  const [soloBajo, setSoloBajo] = useState(false)
  const [accion, setAccion] = useState<Accion | null>(null)
  const productos = useProductos({ tipo: tipo === 'retazos' ? 'placa' : tipo, buscar, categoria })
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
            <DropdownMenuItem onSelect={() => setAccion({ tipo: 'ubicacion', producto: p })}>
              <MapPin /> Ubicación
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

  // Dónde está guardado: con un solo depósito, solo el lugar; con varios, la
  // sucursal adelante.
  const ubicaciones = (p: ProductoConStock) => {
    const partes = depositos
      .filter((d) => p.ubicaciones[d.id])
      .map((d) => (depositos.length > 1 ? `${d.sucursal}: ${p.ubicaciones[d.id]}` : p.ubicaciones[d.id]))
    if (partes.length === 0) return null
    return (
      <div className="flex items-center gap-1 text-xs text-primary">
        <MapPin className="size-3 shrink-0 self-start mt-0.5 md:self-center md:mt-0" /> <span className="md:truncate">{partes.join(' · ')}</span>
      </div>
    )
  }

  const reservado = (n: number) => n > 0 && (
    <div className="text-xs font-normal text-sky-700 dark:text-sky-400">{fmtNumero(n)} reservad{n === 1 ? 'o' : 'os'}</div>
  )

  return (
    <div className="space-y-5">
      <div className="flex flex-wrap items-end justify-between gap-3">
        <div>
          <h2 className="text-2xl font-semibold tracking-tight">Stock</h2>
          <p className="text-muted-foreground">
            Placas, cantos y herrajes en todos los depósitos. Lo reservado ya está pagado y se descuenta cuando el cliente lo retira.
          </p>
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
        {(categorias.length > 0 || categoria) && (
          <Select value={categoria || 'todas'} onValueChange={(v) => setCategoria(v === 'todas' ? '' : v)}>
            <SelectTrigger className="w-full md:w-48" aria-label="Categoría"><SelectValue /></SelectTrigger>
            <SelectContent>
              <SelectItem value="todas">Todas las categorías</SelectItem>
              {categorias.map((c) => <SelectItem key={c.id} value={String(c.id)}>{c.nombre}</SelectItem>)}
              <SelectItem value="sin">Sin categoría</SelectItem>
            </SelectContent>
          </Select>
        )}
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
          <p className="font-medium">{buscar || tipo || soloBajo || categoria ? 'No hay productos que coincidan.' : 'Todavía no cargaste productos.'}</p>
          {editaProductos && !buscar && !tipo && !categoria && (
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
                        {p.categoria && !categoria && <Badge variant="outline" className="font-normal">{p.categoria}</Badge>}
                      </div>
                      <div className="text-xs text-muted-foreground">
                        {[p.sku, detalle(p)].filter(Boolean).join(' · ')}
                      </div>
                      {ubicaciones(p)}
                    </TableCell>
                    {columnas.map((d) => (
                      <TableCell key={d.id} className="text-right">
                        {cantidad(p, p.por_deposito[d.id] ?? 0)}
                        {reservado(p.reservado[d.id] ?? 0)}
                      </TableCell>
                    ))}
                    <TableCell className="text-right font-semibold">
                      <span className={cn('inline-flex items-center gap-1', p.bajo && 'text-amber-700 dark:text-amber-400')}>
                        {p.bajo && <AlertTriangle className="size-3.5" />}{cantidad(p, p.total)}
                      </span>
                      {p.reservado_total > 0 && (
                        <div className="text-xs font-normal text-muted-foreground">{fmtNumero(p.disponible)} libres</div>
                      )}
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
                    {p.categoria && <div className="text-xs text-primary">{p.categoria}</div>}
                    <div className="truncate text-xs text-muted-foreground">{[p.sku, detalle(p)].filter(Boolean).join(' · ')}</div>
                    {ubicaciones(p)}
                  </div>
                  {acciones(p)}
                </div>
                <div className="flex flex-wrap items-center gap-x-4 gap-y-1 text-sm">
                  <span className={cn('font-semibold', p.bajo && 'text-amber-700 dark:text-amber-400')}>
                    {p.bajo && <AlertTriangle className="mr-1 inline size-3.5" />}Total {cantidad(p, p.total)}
                  </span>
                  {p.reservado_total > 0 && (
                    <span className="text-xs text-sky-700 dark:text-sky-400">
                      {fmtNumero(p.reservado_total)} reservados · {fmtNumero(p.disponible)} libres
                    </span>
                  )}
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
      {accion?.tipo === 'ubicacion' && <DialogoUbicacion producto={accion.producto} onClose={() => setAccion(null)} />}
      {accion?.tipo === 'movimientos' && <Movimientos producto={accion.producto} onClose={() => setAccion(null)} />}
    </div>
  )
}
