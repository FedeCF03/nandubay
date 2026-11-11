import { useEffect, useMemo, useState } from 'react'
import { Link, useLocation, useParams } from 'wouter'
import { ArrowLeft, Copy, Loader2, Package, Plus, Trash2 } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Checkbox } from '@/components/ui/checkbox'
import { Input } from '@/components/ui/input'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Skeleton } from '@/components/ui/skeleton'
import { Textarea } from '@/components/ui/textarea'
import { SelectorCantos } from '@/cortes/SelectorCantos'
import type { Cantos } from '@/cortes/tipos'
import { cn } from '@/lib/utils'
import { fmtPesos, unidadCorta, useProductos, type ProductoConStock } from '@/stock/api'
import { BuscarProducto } from '@/ventas/BuscarProducto'
import { SelectorCliente } from '@/ventas/SelectorCliente'
import { type Cliente } from '@/ventas/api'
import {
  RECARGO, sinCantos, tieneCanto, useGuardarPresupuesto, useMateriales, usePresupuesto,
  type Mueble, type PresupuestoMuebles,
} from '@/presupuestos/api'
import { ResumenMateriales } from '@/presupuestos/Materiales'

interface PiezaForm {
  clave: number
  nombre: string
  placa_id: number
  largo: string
  ancho: string
  cantidad: string
  veta: boolean
  cantos: Cantos
  canto_id: number
}

interface InsumoForm { clave: number; producto_id: number; nombre: string; unidad: string; cantidad: string }

interface MuebleForm {
  clave: number
  id?: string // clave del mueble guardado: mantiene sus fotos al editar
  nombre: string
  ancho: string
  alto: string
  prof: string
  cantidad: string
  precio: string
  piezas: PiezaForm[]
  insumos: InsumoForm[]
}

let siguiente = 1
const entero = (s: string) => Number(s.replace(/\D/g, '')) || 0
const decimal = (s: string) => Number(s.replace(/\./g, '').replace(',', '.')) || 0
const soloDigitos = (s: string) => s.replace(/\D/g, '')
const texto = (n: number) => (n ? String(n).replace('.', ',') : '')

const muebleVacio = (): MuebleForm => ({
  clave: siguiente++, nombre: '', ancho: '', alto: '', prof: '', cantidad: '1', precio: '', piezas: [], insumos: [],
})

function desdePresupuesto(p: PresupuestoMuebles, nombres: Map<number, ProductoConStock>): MuebleForm[] {
  return p.muebles.map((m) => ({
    clave: siguiente++, id: m.clave, nombre: m.nombre, ancho: texto(m.ancho_mm), alto: texto(m.alto_mm), prof: texto(m.profundidad_mm),
    cantidad: String(m.cantidad), precio: texto(m.precio_unitario),
    piezas: m.piezas.map((pz) => ({
      clave: siguiente++, nombre: pz.nombre, placa_id: pz.placa_id, largo: String(pz.largo), ancho: String(pz.ancho),
      cantidad: String(pz.cantidad), veta: pz.respeta_veta, cantos: pz.cantos, canto_id: pz.canto_id ?? 0,
    })),
    insumos: m.insumos.map((i) => ({
      clave: siguiente++, producto_id: i.producto_id, cantidad: texto(i.cantidad),
      nombre: nombres.get(i.producto_id)?.nombre ?? `Producto ${i.producto_id}`, unidad: nombres.get(i.producto_id)?.unidad ?? 'unidad',
    })),
  }))
}

// aMuebles pasa el formulario a lo que espera la API. Las piezas todavía sin
// placa o sin medidas se dejan afuera, así el cálculo anda mientras se carga.
function aMuebles(ms: MuebleForm[], precio: (m: MuebleForm, i: number) => number, completas: boolean): Mueble[] {
  return ms.map((m, i) => ({
    clave: m.id,
    nombre: m.nombre.trim() || `Mueble ${i + 1}`,
    ancho_mm: entero(m.ancho), alto_mm: entero(m.alto), profundidad_mm: entero(m.prof),
    cantidad: Math.max(1, entero(m.cantidad)),
    precio_unitario: precio(m, i),
    piezas: m.piezas
      .filter((p) => completas || (p.placa_id && entero(p.largo) && entero(p.ancho) && entero(p.cantidad)))
      .map((p) => ({
        nombre: p.nombre.trim(), placa_id: p.placa_id, largo: entero(p.largo), ancho: entero(p.ancho),
        cantidad: entero(p.cantidad), respeta_veta: p.veta, cantos: p.cantos,
        canto_id: tieneCanto(p.cantos) ? p.canto_id || undefined : undefined,
      })),
    insumos: m.insumos
      .filter((x) => completas || decimal(x.cantidad) > 0)
      .map((x) => ({ producto_id: x.producto_id, cantidad: decimal(x.cantidad) })),
  }))
}

export default function FormPresupuesto() {
  const params = useParams<{ id?: string }>()
  const id = params.id ? Number(params.id) : 0
  const existente = usePresupuesto(id)
  const productos = useProductos({ tipo: '', buscar: '' })
  if ((id && existente.isPending) || productos.isPending) return <Skeleton className="h-96" />
  if (id && existente.isError) return <p className="text-destructive">{existente.error.message}</p>
  return <Formulario presupuesto={id ? existente.data : undefined} productos={productos.data ?? []} />
}

function Formulario({ presupuesto, productos }: { presupuesto?: PresupuestoMuebles; productos: ProductoConStock[] }) {
  const venta = presupuesto?.venta
  const [, navegar] = useLocation()
  const guardar = useGuardarPresupuesto()
  const porId = useMemo(() => new Map(productos.map((p) => [p.id, p])), [productos])
  const placas = productos.filter((p) => p.tipo === 'placa')
  const cantos = productos.filter((p) => p.tipo === 'canto')

  const [cliente, setCliente] = useState<Cliente | null>(venta?.cliente ?? null)
  const [muebles, setMuebles] = useState<MuebleForm[]>(() =>
    presupuesto ? desdePresupuesto(presupuesto, porId) : [muebleVacio()])
  const [notas, setNotas] = useState(venta?.notas ?? '')
  const [entrega, setEntrega] = useState(venta?.fecha_entrega ?? '')
  const [buscandoPara, setBuscandoPara] = useState<number | null>(null)
  const [error, setError] = useState('')

  useEffect(() => { setError('') }, [cliente, muebles])

  // El cálculo espera a que se deje de escribir un momento.
  const paraCalcular = useMemo(() => aMuebles(muebles, () => 0, false), [muebles])
  const [demorado, setDemorado] = useState(paraCalcular)
  useEffect(() => {
    const t = setTimeout(() => setDemorado(paraCalcular), 400)
    return () => clearTimeout(t)
  }, [paraCalcular])
  const hayPiezas = demorado.some((m) => m.piezas.length || m.insumos.length)
  const materiales = useMateriales(hayPiezas ? demorado : null)
  const mat = hayPiezas ? materiales.data : undefined

  // El precio es siempre el costo × RECARGO (lo calcula el backend). Solo un
  // mueble sin piezas ni herrajes lleva precio a mano.
  const sinMateriales = (m: MuebleForm) => m.piezas.length === 0 && m.insumos.length === 0
  const calculado = (i: number) => mat?.precio_por_mueble?.[i] ?? 0
  const precioDe = (m: MuebleForm, i: number) => (sinMateriales(m) ? decimal(m.precio) : calculado(i))
  const total = muebles.reduce((t, m, i) => t + precioDe(m, i) * Math.max(1, entero(m.cantidad)), 0)

  const cambiarMueble = (clave: number, f: (m: MuebleForm) => MuebleForm) =>
    setMuebles((ms) => ms.map((m) => (m.clave === clave ? f(m) : m)))
  const cambiarPieza = (mc: number, pc: number, cambios: Partial<PiezaForm>) =>
    cambiarMueble(mc, (m) => ({ ...m, piezas: m.piezas.map((p) => (p.clave === pc ? { ...p, ...cambios } : p)) }))

  const agregarPieza = (m: MuebleForm) => {
    // La pieza nueva usa la placa y el canto de la anterior: así se cargan
    // rápido varias piezas del mismo color.
    const ultima = m.piezas[m.piezas.length - 1]
    const placa = ultima?.placa_id ?? placas[0]?.id ?? 0
    cambiarMueble(m.clave, (x) => ({
      ...x,
      piezas: [...x.piezas, {
        clave: siguiente++, nombre: '', placa_id: placa, largo: '', ancho: '', cantidad: '1',
        veta: porId.get(placa)?.tiene_veta ?? false, cantos: sinCantos(), canto_id: ultima?.canto_id || cantos[0]?.id || 0,
      }],
    }))
  }

  const agregarInsumo = (p: ProductoConStock) => {
    const mc = buscandoPara
    setBuscandoPara(null)
    if (mc == null) return
    cambiarMueble(mc, (m) => m.insumos.some((i) => i.producto_id === p.id) ? m : ({
      ...m, insumos: [...m.insumos, { clave: siguiente++, producto_id: p.id, nombre: p.nombre, unidad: p.unidad, cantidad: '1' }],
    }))
  }

  const duplicar = (m: MuebleForm) => setMuebles((ms) => {
    const i = ms.findIndex((x) => x.clave === m.clave)
    const copia: MuebleForm = {
      ...m, clave: siguiente++, id: undefined, nombre: m.nombre ? `${m.nombre} (copia)` : '',
      piezas: m.piezas.map((p) => ({ ...p, clave: siguiente++ })),
      insumos: m.insumos.map((x) => ({ ...x, clave: siguiente++ })),
    }
    return [...ms.slice(0, i + 1), copia, ...ms.slice(i + 1)]
  })

  const enviar = () => {
    if (!cliente) return setError('Elegí el cliente.')
    if (muebles.length === 0) return setError('Agregá al menos un mueble.')
    const sinPrecio = muebles.findIndex((m, i) => sinMateriales(m) && precioDe(m, i) <= 0)
    if (sinPrecio >= 0) return setError(`Poné el precio de ${muebles[sinPrecio].nombre || `el mueble ${sinPrecio + 1}`}.`)
    guardar.mutate({
      id: venta?.id,
      datos: { cliente_id: cliente.id, notas, fecha_entrega: entrega || null, muebles: aMuebles(muebles, precioDe, true) },
    }, { onSuccess: (p) => navegar(`/presupuestos/${p.venta.id}`) })
  }

  const volver = venta ? `/presupuestos/${venta.id}` : '/presupuestos'

  return (
    <div className="mx-auto max-w-6xl space-y-5">
      <div className="flex items-center gap-2">
        <Button variant="ghost" size="icon" asChild aria-label="Volver"><Link href={volver}><ArrowLeft /></Link></Button>
        <h2 className="text-2xl font-semibold tracking-tight">
          {venta ? `Editar presupuesto N° ${venta.numero}` : 'Nuevo presupuesto de muebles'}
        </h2>
      </div>

      <div className="grid items-start gap-5 lg:grid-cols-[1fr_21rem]">
        <div className="min-w-0 space-y-5">
          <Card>
            <CardHeader><CardTitle>Cliente</CardTitle></CardHeader>
            <CardContent><SelectorCliente valor={cliente} onChange={setCliente} /></CardContent>
          </Card>

          {muebles.map((m, i) => (
            <Card key={m.clave}>
              <CardHeader className="flex flex-row items-start justify-between gap-2">
                <CardTitle className="pt-2">Mueble {i + 1}</CardTitle>
                <div className="flex">
                  <Button type="button" variant="ghost" size="icon" aria-label={`Duplicar mueble ${i + 1}`} title="Duplicar"
                    onClick={() => duplicar(m)}><Copy /></Button>
                  <Button type="button" variant="ghost" size="icon" aria-label={`Quitar mueble ${i + 1}`} title="Quitar"
                    className="text-muted-foreground" onClick={() => setMuebles((ms) => ms.filter((x) => x.clave !== m.clave))}><Trash2 /></Button>
                </div>
              </CardHeader>
              <CardContent className="space-y-5">
                <div className="grid gap-3 sm:grid-cols-[1fr_5rem_9rem]">
                  <label className="grid gap-1.5 text-sm font-medium">
                    Nombre
                    <Input value={m.nombre} placeholder="Ej: Bajo mesada 2 puertas"
                      onChange={(e) => cambiarMueble(m.clave, (x) => ({ ...x, nombre: e.target.value }))} />
                  </label>
                  <label className="grid gap-1.5 text-sm font-medium">
                    Cantidad
                    <Input value={m.cantidad} inputMode="numeric" className="text-right"
                      onChange={(e) => cambiarMueble(m.clave, (x) => ({ ...x, cantidad: soloDigitos(e.target.value) }))} />
                  </label>
                  <label className="grid gap-1.5 text-sm font-medium">
                    Precio unitario
                    {sinMateriales(m) ? (
                      <Input value={m.precio} inputMode="decimal" className="text-right" aria-label={`Precio del mueble ${i + 1}`}
                        onChange={(e) => cambiarMueble(m.clave, (x) => ({ ...x, precio: e.target.value }))} />
                    ) : (
                      <Input value={calculado(i) ? fmtPesos(calculado(i)) : '…'} readOnly tabIndex={-1} aria-label={`Precio del mueble ${i + 1}`}
                        title="Costo de materiales × 1,7" className="bg-muted text-right" />
                    )}
                  </label>
                </div>
                <div className="grid grid-cols-3 gap-2 sm:max-w-md">
                  <Input value={m.ancho} inputMode="numeric" placeholder="Ancho mm" aria-label="Ancho en mm"
                    onChange={(e) => cambiarMueble(m.clave, (x) => ({ ...x, ancho: soloDigitos(e.target.value) }))} />
                  <Input value={m.alto} inputMode="numeric" placeholder="Alto mm" aria-label="Alto en mm"
                    onChange={(e) => cambiarMueble(m.clave, (x) => ({ ...x, alto: soloDigitos(e.target.value) }))} />
                  <Input value={m.prof} inputMode="numeric" placeholder="Prof. mm" aria-label="Profundidad en mm"
                    onChange={(e) => cambiarMueble(m.clave, (x) => ({ ...x, prof: soloDigitos(e.target.value) }))} />
                </div>

                <section className="space-y-2">
                  <div className="flex items-center justify-between">
                    <h3 className="font-medium">Piezas a cortar</h3>
                    <Button type="button" variant="outline" size="sm" onClick={() => agregarPieza(m)} disabled={placas.length === 0}>
                      <Plus /> Pieza
                    </Button>
                  </div>
                  {placas.length === 0 && (
                    <p className="text-sm text-muted-foreground">Primero cargá las placas en <Link href="/stock" className="underline">Stock</Link>.</p>
                  )}
                  {m.piezas.length === 0 && placas.length > 0 && (
                    <p className="rounded-lg border border-dashed py-6 text-center text-sm text-muted-foreground">
                      Agregá laterales, estantes, puertas… Cada pieza puede ser de una placa distinta.
                    </p>
                  )}
                  {m.piezas.map((p, j) => {
                    const placa = porId.get(p.placa_id)
                    return (
                      <div key={p.clave}
                        className="grid grid-cols-[1fr_1fr_4.5rem] gap-2 rounded-lg border p-2 md:grid-cols-[minmax(6rem,1fr)_minmax(8rem,1.3fr)_4.5rem_4.5rem_3.5rem_auto_auto] md:items-center md:border-0 md:p-0">
                        <Input value={p.nombre} placeholder={`Pieza ${j + 1}`} aria-label={`Nombre de la pieza ${j + 1} (opcional)`}
                          className="col-span-3 md:col-span-1" onChange={(e) => cambiarPieza(m.clave, p.clave, { nombre: e.target.value })} />
                        <Select value={p.placa_id ? String(p.placa_id) : ''}
                          onValueChange={(v) => cambiarPieza(m.clave, p.clave, { placa_id: Number(v), veta: porId.get(Number(v))?.tiene_veta ?? false })}>
                          <SelectTrigger className="col-span-3 w-full md:col-span-1" aria-label={`Placa de la pieza ${j + 1}`}>
                            <SelectValue placeholder="Placa" />
                          </SelectTrigger>
                          <SelectContent>
                            {placas.map((pl) => <SelectItem key={pl.id} value={String(pl.id)}>{pl.nombre}</SelectItem>)}
                          </SelectContent>
                        </Select>
                        <Input value={p.largo} inputMode="numeric" placeholder="Largo" aria-label={`Largo de la pieza ${j + 1}`}
                          className="text-right" onChange={(e) => cambiarPieza(m.clave, p.clave, { largo: soloDigitos(e.target.value) })} />
                        <Input value={p.ancho} inputMode="numeric" placeholder="Ancho" aria-label={`Ancho de la pieza ${j + 1}`}
                          className="text-right" onChange={(e) => cambiarPieza(m.clave, p.clave, { ancho: soloDigitos(e.target.value) })} />
                        <Input value={p.cantidad} inputMode="numeric" aria-label={`Cantidad de la pieza ${j + 1}`} title="Cantidad por mueble"
                          className="text-right" onChange={(e) => cambiarPieza(m.clave, p.clave, { cantidad: soloDigitos(e.target.value) })} />
                        <div className="col-span-2 flex items-center gap-3 md:col-span-1">
                          <SelectorCantos valor={p.cantos} onChange={(c) => cambiarPieza(m.clave, p.clave, { cantos: c })} />
                          {placa?.tiene_veta && (
                            <label className="flex items-center gap-1.5 text-xs whitespace-nowrap">
                              <Checkbox checked={p.veta} onCheckedChange={(v) => cambiarPieza(m.clave, p.clave, { veta: v === true })} />
                              Veta
                            </label>
                          )}
                        </div>
                        <Button type="button" variant="ghost" size="icon" aria-label={`Quitar pieza ${j + 1}`}
                          className="justify-self-end text-muted-foreground"
                          onClick={() => cambiarMueble(m.clave, (x) => ({ ...x, piezas: x.piezas.filter((y) => y.clave !== p.clave) }))}>
                          <Trash2 />
                        </Button>
                        {tieneCanto(p.cantos) && (
                          <Select value={p.canto_id ? String(p.canto_id) : ''}
                            onValueChange={(v) => cambiarPieza(m.clave, p.clave, { canto_id: Number(v) })}>
                            <SelectTrigger size="sm" aria-label={`Canto de la pieza ${j + 1}`}
                              className={cn('col-span-3 w-full md:col-span-2 md:col-start-2', !p.canto_id && 'border-destructive')}>
                              <SelectValue placeholder={cantos.length ? 'Elegí el canto' : 'No hay cantos en el stock'} />
                            </SelectTrigger>
                            <SelectContent>
                              {cantos.map((c) => <SelectItem key={c.id} value={String(c.id)}>{c.nombre}</SelectItem>)}
                            </SelectContent>
                          </Select>
                        )}
                      </div>
                    )
                  })}
                  {m.piezas.length > 0 && (
                    <p className="text-xs text-muted-foreground">Medidas en mm. La cantidad es por mueble. Tocá los bordes del dibujo para marcar el canto.</p>
                  )}
                </section>

                <section className="space-y-2">
                  <div className="flex items-center justify-between">
                    <h3 className="font-medium">Herrajes e insumos</h3>
                    <Button type="button" variant="outline" size="sm" onClick={() => setBuscandoPara(m.clave)}><Package /> Agregar</Button>
                  </div>
                  {m.insumos.length === 0 ? (
                    <p className="text-sm text-muted-foreground">Bisagras, guías, patas regulables, tornillos…</p>
                  ) : (
                    <ul className="space-y-2">
                      {m.insumos.map((x) => (
                        <li key={x.clave} className="flex items-center gap-2">
                          <span className="min-w-0 flex-1 truncate text-sm">{x.nombre}</span>
                          <Input value={x.cantidad} inputMode="decimal" className="w-20 text-right" aria-label={`Cantidad de ${x.nombre}`}
                            onChange={(e) => cambiarMueble(m.clave, (y) => ({
                              ...y, insumos: y.insumos.map((z) => (z.clave === x.clave ? { ...z, cantidad: e.target.value } : z)),
                            }))} />
                          <span className="w-6 text-xs text-muted-foreground">{unidadCorta[x.unidad] ?? x.unidad}</span>
                          <Button type="button" variant="ghost" size="icon" aria-label={`Quitar ${x.nombre}`} className="text-muted-foreground"
                            onClick={() => cambiarMueble(m.clave, (y) => ({ ...y, insumos: y.insumos.filter((z) => z.clave !== x.clave) }))}>
                            <Trash2 />
                          </Button>
                        </li>
                      ))}
                    </ul>
                  )}
                </section>
              </CardContent>
            </Card>
          ))}

          <Button type="button" variant="outline" className="w-full border-dashed" onClick={() => setMuebles((ms) => [...ms, muebleVacio()])}>
            <Plus /> Agregar mueble
          </Button>

          <Card>
            <CardHeader><CardTitle>Entrega y notas</CardTitle></CardHeader>
            <CardContent className="grid gap-4 sm:grid-cols-[12rem_1fr]">
              <label className="grid gap-1.5 text-sm font-medium">
                Fecha de entrega
                <Input type="date" value={entrega} onChange={(e) => setEntrega(e.target.value)} />
              </label>
              <label className="grid gap-1.5 text-sm font-medium">
                Notas (salen en el presupuesto)
                <Textarea value={notas} onChange={(e) => setNotas(e.target.value)} rows={2} />
              </label>
            </CardContent>
          </Card>
        </div>

        <div className="space-y-4 lg:sticky lg:top-4">
          <Card>
            <CardHeader className="flex flex-row items-center justify-between">
              <CardTitle>Materiales</CardTitle>
              {materiales.isFetching && <Loader2 className="size-4 animate-spin text-muted-foreground" />}
            </CardHeader>
            <CardContent className="space-y-4">
              {!hayPiezas ? (
                <p className="text-sm text-muted-foreground">Cargá piezas y herrajes y acá se calculan las placas, el canto y el costo.</p>
              ) : materiales.isError ? (
                <p className="text-sm text-destructive">{materiales.error.message}</p>
              ) : mat ? (
                <ResumenMateriales mat={mat} />
              ) : (
                <Skeleton className="h-32" />
              )}
              <p className="border-t pt-3 text-xs text-muted-foreground">
                Precio de cada mueble: costo de materiales × {String(RECARGO).replace('.', ',')}, con las placas enteras.
              </p>
              <div className="flex items-center justify-between border-t pt-3 text-lg">
                <span className="text-muted-foreground">Total</span>
                <span className="font-semibold tabular-nums">{fmtPesos(total)}</span>
              </div>
            </CardContent>
          </Card>

          {(error || guardar.error) && <p className="text-sm text-destructive">{error || guardar.error?.message}</p>}
          <div className="flex justify-end gap-2 pb-6">
            <Button variant="outline" asChild><Link href={volver}>Cancelar</Link></Button>
            <Button onClick={enviar} disabled={guardar.isPending}>
              {guardar.isPending ? <Loader2 className="animate-spin" /> : <Plus />} {venta ? 'Guardar cambios' : 'Guardar presupuesto'}
            </Button>
          </div>
        </div>
      </div>

      {buscandoPara != null && (
        <BuscarProducto sinPlacas onElegir={agregarInsumo} onClose={() => setBuscandoPara(null)}
          titulo="Agregar herraje o insumo" descripcion="Bisagras, guías, patas y otros productos del stock." />
      )}
    </div>
  )
}
