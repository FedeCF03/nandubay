import { useMemo, useState } from 'react'
import { Link, useParams } from 'wouter'
import { ArrowLeft, Loader2, Package, Plus, Printer, Trash2 } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Checkbox } from '@/components/ui/checkbox'
import { Input } from '@/components/ui/input'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Skeleton } from '@/components/ui/skeleton'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import { usePermiso } from '@/auth/sesion'
import { SelectorCantos } from '@/cortes/SelectorCantos'
import type { Cantos } from '@/cortes/tipos'
import { cn } from '@/lib/utils'
import { fmtNumero, fmtPesos, unidadCorta, useProductos, type ProductoConStock } from '@/stock/api'
import { BuscarProducto } from '@/ventas/BuscarProducto'
import { medidas, numeroVenta } from '@/ventas/api'
import { FotosMueble } from '@/presupuestos/Fotos'
import { PlanesDeCorte, ResumenMateriales } from '@/presupuestos/Materiales'
import {
  RECARGO, sinCantos, tieneCanto, useGuardarPresupuesto, useMateriales, usePresupuesto,
  type Mueble, type PiezaMueble, type PresupuestoMuebles,
} from '@/presupuestos/api'

const lados = (c: Cantos) => (['l1', 'l2', 'a1', 'a2'] as const).filter((k) => c[k]).map((k) => k.toUpperCase()).join(' ')
const soloDigitos = (s: string) => s.replace(/\D/g, '')

// MuebleDetalle es un mueble dentro del presupuesto: sus piezas, su hoja de
// cortes, lo que usa de canto y herrajes, y (si el presupuesto todavía se puede
// editar) agregarle o quitarle piezas y herrajes.
export default function MuebleDetalle() {
  const { id, n } = useParams<{ id: string; n: string }>()
  const p = usePresupuesto(Number(id))
  const productos = useProductos({ tipo: '', buscar: '' })
  if (p.isPending || productos.isPending) return <Skeleton className="h-96" />
  if (p.isError) return <p className="text-destructive">{p.error.message}</p>
  const i = Number(n) - 1
  if (!p.data.muebles[i]) return <p className="text-destructive">Ese mueble no está en el presupuesto.</p>
  return <Detalle p={p.data} i={i} productos={productos.data ?? []} />
}

function Detalle({ p, i, productos }: { p: PresupuestoMuebles; i: number; productos: ProductoConStock[] }) {
  const v = p.venta
  const m = p.muebles[i]
  const puedeEditar = usePermiso('pedidos.editar')
  const edita = puedeEditar && v.estado === 'presupuesto'
  const guardar = useGuardarPresupuesto()
  const porId = useMemo(() => new Map(productos.map((x) => [x.id, x])), [productos])
  const [buscando, setBuscando] = useState(false)

  // Materiales de este mueble solo: su hoja de cortes y lo que usa.
  const solo = useMemo(() => [m], [m])
  const hay = m.piezas.length > 0 || m.insumos.length > 0
  const mat = useMateriales(hay ? solo : null)

  const guardarMueble = (nuevo: Mueble) => guardar.mutate({
    id: v.id,
    datos: {
      cliente_id: v.cliente.id, notas: v.notas, fecha_entrega: v.fecha_entrega,
      muebles: p.muebles.map((x, j) => (j === i ? nuevo : x)),
    },
  })
  const agregarInsumo = (pr: ProductoConStock) => {
    setBuscando(false)
    if (m.insumos.some((x) => x.producto_id === pr.id)) return
    guardarMueble({ ...m, insumos: [...m.insumos, { producto_id: pr.id, cantidad: 1 }] })
  }

  const base = `/presupuestos/${v.id}`
  return (
    <div className="mx-auto max-w-6xl space-y-5">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div className="flex items-center gap-2">
          <Button variant="ghost" size="icon" asChild aria-label="Volver al presupuesto"><Link href={base}><ArrowLeft /></Link></Button>
          <div>
            <h2 className="text-2xl font-semibold tracking-tight">{m.cantidad > 1 && `${m.cantidad} × `}{m.nombre}</h2>
            <p className="text-sm text-muted-foreground">
              <Link href={base} className="hover:underline">Presupuesto N° {numeroVenta(v.numero)}</Link> · {v.cliente.nombre}
              {medidas(m) && ` · ${medidas(m)}`}
            </p>
          </div>
        </div>
        <div className="flex gap-2">
          {guardar.isPending && <Loader2 className="size-5 animate-spin self-center text-muted-foreground" />}
          {m.piezas.length > 0 && (
            <Button variant="outline" asChild><Link href={`${base}/cortes?mueble=${i + 1}`}><Printer /> Hoja de cortes</Link></Button>
          )}
        </div>
      </div>
      {guardar.error && <p className="text-sm text-destructive">{guardar.error.message}</p>}

      <div className="grid items-start gap-5 lg:grid-cols-[minmax(0,1fr)_21rem]">
        <div className="min-w-0 space-y-5">
          <Card className="gap-3">
            <CardHeader><CardTitle>Piezas</CardTitle></CardHeader>
            <CardContent className="space-y-4">
              {m.piezas.length === 0 ? (
                <p className="text-sm text-muted-foreground">Todavía no tiene piezas.</p>
              ) : (
                <div className="-mx-6 overflow-x-auto">
                  <Table>
                    <TableHeader>
                      <TableRow>
                        <TableHead className="pl-6">Pieza</TableHead>
                        <TableHead>Placa</TableHead>
                        <TableHead className="text-right">Medida</TableHead>
                        <TableHead className="text-right">Cant.</TableHead>
                        <TableHead>Canto</TableHead>
                        {edita && <TableHead className="w-10 pr-6" />}
                      </TableRow>
                    </TableHeader>
                    <TableBody>
                      {m.piezas.map((pz, j) => (
                        <TableRow key={j}>
                          <TableCell className="pl-6 font-medium">{pz.nombre || `Pieza ${j + 1}`}</TableCell>
                          <TableCell className="max-w-40 truncate">{porId.get(pz.placa_id)?.nombre ?? '—'}</TableCell>
                          <TableCell className="text-right whitespace-nowrap tabular-nums">{pz.largo} × {pz.ancho}</TableCell>
                          <TableCell className="text-right tabular-nums">{pz.cantidad}</TableCell>
                          <TableCell className="text-sm">
                            {tieneCanto(pz.cantos)
                              ? <><span className="font-mono">{lados(pz.cantos)}</span> <span className="text-muted-foreground">{porId.get(pz.canto_id ?? 0)?.nombre}</span></>
                              : <span className="text-muted-foreground">—</span>}
                          </TableCell>
                          {edita && (
                            <TableCell className="pr-6">
                              <Button variant="ghost" size="icon" className="text-muted-foreground" aria-label={`Quitar ${pz.nombre || `pieza ${j + 1}`}`}
                                disabled={guardar.isPending}
                                onClick={() => guardarMueble({ ...m, piezas: m.piezas.filter((_, k) => k !== j) })}>
                                <Trash2 />
                              </Button>
                            </TableCell>
                          )}
                        </TableRow>
                      ))}
                    </TableBody>
                  </Table>
                </div>
              )}
              {edita && (
                <NuevaPieza productos={productos} anterior={m.piezas[m.piezas.length - 1]} guardando={guardar.isPending}
                  onAgregar={(pz) => guardarMueble({ ...m, piezas: [...m.piezas, pz] })} />
              )}
            </CardContent>
          </Card>

          <Card className="gap-3">
            <CardHeader className="flex flex-row items-center justify-between">
              <CardTitle>Herrajes e insumos</CardTitle>
              {edita && <Button variant="outline" size="sm" onClick={() => setBuscando(true)}><Package /> Agregar</Button>}
            </CardHeader>
            <CardContent>
              {m.insumos.length === 0 ? (
                <p className="text-sm text-muted-foreground">Bisagras, guías, patas regulables, tornillos…</p>
              ) : (
                <ul className="divide-y text-sm">
                  {m.insumos.map((x, j) => {
                    const pr = porId.get(x.producto_id)
                    return (
                      <li key={x.producto_id} className="flex items-center gap-2 py-1.5">
                        <span className="min-w-0 flex-1 truncate">{pr?.nombre ?? `Producto ${x.producto_id}`}</span>
                        {edita ? (
                          <CantidadInsumo valor={x.cantidad} nombre={pr?.nombre ?? ''} disabled={guardar.isPending}
                            onChange={(c) => guardarMueble({ ...m, insumos: m.insumos.map((y, k) => (k === j ? { ...y, cantidad: c } : y)) })} />
                        ) : (
                          <span className="tabular-nums">{fmtNumero(x.cantidad)}</span>
                        )}
                        <span className="w-6 text-xs text-muted-foreground">{pr ? unidadCorta[pr.unidad] : ''}</span>
                        {edita && (
                          <Button variant="ghost" size="icon" className="text-muted-foreground" aria-label={`Quitar ${pr?.nombre ?? 'insumo'}`}
                            disabled={guardar.isPending}
                            onClick={() => guardarMueble({ ...m, insumos: m.insumos.filter((_, k) => k !== j) })}>
                            <Trash2 />
                          </Button>
                        )}
                      </li>
                    )
                  })}
                </ul>
              )}
              {m.cantidad > 1 && m.insumos.length > 0 && <p className="mt-2 text-xs text-muted-foreground">Cantidades por mueble.</p>}
            </CardContent>
          </Card>

          {m.clave && (
            <Card>
              <CardContent>
                <FotosMueble pedidoId={v.id} clave={m.clave} nombre={m.nombre} edita={puedeEditar}
                  fotos={p.fotos.filter((f) => f.mueble_clave === m.clave)} />
              </CardContent>
            </Card>
          )}
        </div>

        <Card className="lg:sticky lg:top-4">
          <CardHeader className="flex flex-row items-center justify-between">
            <CardTitle>Lo que usa {m.cantidad > 1 ? `(${m.cantidad} muebles)` : ''}</CardTitle>
            {mat.isFetching && <Loader2 className="size-4 animate-spin text-muted-foreground" />}
          </CardHeader>
          <CardContent className="space-y-4">
            {!hay ? <p className="text-sm text-muted-foreground">Agregá piezas y herrajes.</p>
              : mat.isError ? <p className="text-sm text-destructive">{mat.error.message}</p>
                : mat.data ? <ResumenMateriales mat={mat.data} /> : <Skeleton className="h-32" />}
            <div className="space-y-1 border-t pt-3 text-sm">
              <div className="flex justify-between"><span className="text-muted-foreground">Precio por mueble</span><b className="tabular-nums">{fmtPesos(m.precio_unitario)}</b></div>
              <p className="text-xs text-muted-foreground">
                Costo de materiales × {String(RECARGO).replace('.', ',')}. Si el mueble va junto con otros, las placas se reparten entre todos y se cobran enteras.
              </p>
            </div>
          </CardContent>
        </Card>
      </div>

      {mat.data && mat.data.placas.length > 0 && (
        <Card>
          <CardHeader><CardTitle>Hoja de cortes de este mueble</CardTitle></CardHeader>
          <CardContent><PlanesDeCorte mat={mat.data} muebles={solo} /></CardContent>
        </Card>
      )}

      {buscando && (
        <BuscarProducto sinPlacas onElegir={agregarInsumo} onClose={() => setBuscando(false)}
          titulo="Agregar herraje o insumo" descripcion="Bisagras, guías, patas y otros productos del stock." />
      )}
    </div>
  )
}

// CantidadInsumo guarda la cantidad al salir del campo, no en cada tecla.
function CantidadInsumo({ valor, nombre, disabled, onChange }: { valor: number; nombre: string; disabled: boolean; onChange: (n: number) => void }) {
  const [texto, setTexto] = useState(String(valor).replace('.', ','))
  const confirmar = () => {
    const n = Number(texto.replace(/\./g, '').replace(',', '.'))
    if (n > 0 && n !== valor) onChange(n)
    else setTexto(String(valor).replace('.', ','))
  }
  return (
    <Input value={texto} inputMode="decimal" className="h-8 w-20 text-right" aria-label={`Cantidad de ${nombre}`} disabled={disabled}
      onChange={(e) => setTexto(e.target.value)} onBlur={confirmar} onKeyDown={(e) => e.key === 'Enter' && confirmar()} />
  )
}

// NuevaPieza es la fila para agregar una pieza al mueble. Arranca con la
// placa y el canto de la última pieza.
function NuevaPieza({ productos, anterior, guardando, onAgregar }: {
  productos: ProductoConStock[]
  anterior?: PiezaMueble
  guardando: boolean
  onAgregar: (p: PiezaMueble) => void
}) {
  const placas = productos.filter((x) => x.tipo === 'placa')
  const cantos = productos.filter((x) => x.tipo === 'canto')
  const porId = new Map(productos.map((x) => [x.id, x]))
  const inicial = () => {
    const placa = anterior?.placa_id ?? placas[0]?.id ?? 0
    return {
      nombre: '', placa_id: placa, largo: '', ancho: '', cantidad: '1',
      veta: porId.get(placa)?.tiene_veta ?? false, cantos: sinCantos(), canto_id: anterior?.canto_id || cantos[0]?.id || 0,
    }
  }
  const [f, setF] = useState(inicial)
  const [error, setError] = useState('')
  const cambiar = (c: Partial<typeof f>) => { setF((x) => ({ ...x, ...c })); setError('') }
  const placa = porId.get(f.placa_id)

  const agregar = () => {
    const largo = Number(f.largo), ancho = Number(f.ancho), cant = Number(f.cantidad)
    if (!f.placa_id) return setError('Elegí la placa.')
    if (!largo || !ancho || !cant) return setError('Completá largo, ancho y cantidad.')
    if (tieneCanto(f.cantos) && !f.canto_id) return setError('Elegí el canto.')
    onAgregar({
      nombre: f.nombre.trim(), placa_id: f.placa_id, largo, ancho, cantidad: cant, respeta_veta: f.veta, cantos: f.cantos,
      canto_id: tieneCanto(f.cantos) ? f.canto_id : undefined,
    })
    setF({ ...inicial(), placa_id: f.placa_id, veta: f.veta, canto_id: f.canto_id })
  }

  if (placas.length === 0) return <p className="text-sm text-muted-foreground">Primero cargá las placas en <Link href="/stock" className="underline">Stock</Link>.</p>
  return (
    <div className="space-y-2 rounded-lg border border-dashed p-3">
      <div className="text-sm font-medium">Agregar pieza</div>
      <div className="grid grid-cols-3 gap-2 md:grid-cols-[minmax(6rem,1fr)_minmax(8rem,1.3fr)_4.5rem_4.5rem_3.5rem_auto] md:items-center">
        <Input value={f.nombre} placeholder="Nombre (opcional)" aria-label="Nombre de la pieza nueva" className="col-span-3 md:col-span-1"
          onChange={(e) => cambiar({ nombre: e.target.value })} />
        <Select value={f.placa_id ? String(f.placa_id) : ''}
          onValueChange={(x) => cambiar({ placa_id: Number(x), veta: porId.get(Number(x))?.tiene_veta ?? false })}>
          <SelectTrigger className="col-span-3 w-full md:col-span-1" aria-label="Placa de la pieza nueva"><SelectValue placeholder="Placa" /></SelectTrigger>
          <SelectContent>{placas.map((pl) => <SelectItem key={pl.id} value={String(pl.id)}>{pl.nombre}</SelectItem>)}</SelectContent>
        </Select>
        <Input value={f.largo} inputMode="numeric" placeholder="Largo" aria-label="Largo de la pieza nueva" className="text-right"
          onChange={(e) => cambiar({ largo: soloDigitos(e.target.value) })} />
        <Input value={f.ancho} inputMode="numeric" placeholder="Ancho" aria-label="Ancho de la pieza nueva" className="text-right"
          onChange={(e) => cambiar({ ancho: soloDigitos(e.target.value) })} />
        <Input value={f.cantidad} inputMode="numeric" aria-label="Cantidad de la pieza nueva" title="Cantidad por mueble" className="text-right"
          onChange={(e) => cambiar({ cantidad: soloDigitos(e.target.value) })} />
        <div className="col-span-3 flex items-center gap-3 md:col-span-1">
          <SelectorCantos valor={f.cantos} onChange={(c) => cambiar({ cantos: c })} />
          {placa?.tiene_veta && (
            <label className="flex items-center gap-1.5 text-xs whitespace-nowrap">
              <Checkbox checked={f.veta} onCheckedChange={(x) => cambiar({ veta: x === true })} /> Veta
            </label>
          )}
        </div>
        {tieneCanto(f.cantos) && (
          <Select value={f.canto_id ? String(f.canto_id) : ''} onValueChange={(x) => cambiar({ canto_id: Number(x) })}>
            <SelectTrigger size="sm" aria-label="Canto de la pieza nueva"
              className={cn('col-span-3 w-full md:col-span-2 md:col-start-2', !f.canto_id && 'border-destructive')}>
              <SelectValue placeholder={cantos.length ? 'Elegí el canto' : 'No hay cantos en el stock'} />
            </SelectTrigger>
            <SelectContent>{cantos.map((c) => <SelectItem key={c.id} value={String(c.id)}>{c.nombre}</SelectItem>)}</SelectContent>
          </Select>
        )}
      </div>
      <div className="flex items-center justify-between gap-2">
        <span className="text-sm text-destructive">{error}</span>
        <Button size="sm" onClick={agregar} disabled={guardando}><Plus /> Agregar pieza</Button>
      </div>
    </div>
  )
}
