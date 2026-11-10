import { useEffect, useMemo, useState } from 'react'
import { Link, useLocation, useParams } from 'wouter'
import { ArrowLeft, Hammer, Loader2, Package, Plus, Search, Trash2 } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import {
  Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle,
} from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Skeleton } from '@/components/ui/skeleton'
import { Textarea } from '@/components/ui/textarea'
import { detalle, fmtNumero, fmtPesos, unidadCorta, useProductos, type ProductoConStock } from '@/stock/api'
import { useGuardarVenta, useVenta, type Cliente, type Venta } from '@/ventas/api'
import { SelectorCliente } from '@/ventas/SelectorCliente'

interface LineaForm {
  clave: number
  tipo: 'producto' | 'mueble'
  producto_id?: number
  descripcion: string
  unidad: string
  ancho: string
  alto: string
  prof: string
  cantidad: string
  precio: string
}

let siguiente = 1
const num = (s: string) => Number(s.replace(/\./g, '').replace(',', '.'))
const numSimple = (s: string) => Number(s.replace(',', '.'))
const texto = (n?: number) => (n ? String(n).replace('.', ',') : '')

function desdeVenta(v: Venta): LineaForm[] {
  return v.lineas.map((l) => ({
    clave: siguiente++, tipo: l.tipo, producto_id: l.producto_id, descripcion: l.descripcion, unidad: l.unidad,
    ancho: texto(l.ancho_mm), alto: texto(l.alto_mm), prof: texto(l.profundidad_mm),
    cantidad: texto(l.cantidad), precio: String(l.precio_unitario).replace('.', ','),
  }))
}

function BuscarProducto({ onElegir, onClose }: { onElegir: (p: ProductoConStock) => void; onClose: () => void }) {
  const [buscar, setBuscar] = useState('')
  const productos = useProductos({ tipo: '', buscar })
  return (
    <Dialog open onOpenChange={(o) => !o && onClose()}>
      <DialogContent className="max-h-[85svh] grid-rows-[auto_auto_1fr]">
        <DialogHeader>
          <DialogTitle>Agregar producto</DialogTitle>
          <DialogDescription>Placas, cantos o herrajes del stock.</DialogDescription>
        </DialogHeader>
        <div className="relative">
          <Search className="pointer-events-none absolute top-1/2 left-2.5 size-4 -translate-y-1/2 text-muted-foreground" />
          <Input className="pl-8" autoFocus placeholder="Nombre, código o color" value={buscar}
            onChange={(e) => setBuscar(e.target.value)} aria-label="Buscar producto" />
        </div>
        <div className="-mx-2 min-h-0 overflow-y-auto">
          {productos.isPending ? (
            <div className="space-y-2 px-2">{Array.from({ length: 4 }, (_, i) => <Skeleton key={i} className="h-12" />)}</div>
          ) : (productos.data ?? []).length === 0 ? (
            <p className="px-2 py-6 text-center text-sm text-muted-foreground">No hay productos que coincidan.</p>
          ) : (
            (productos.data ?? []).map((p) => (
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

export default function FormVenta() {
  const params = useParams<{ id?: string }>()
  const id = params.id ? Number(params.id) : undefined
  const existente = useVenta(id ?? 0)
  if (id && existente.isPending) return <Skeleton className="h-96" />
  if (id && existente.isError) return <p className="text-destructive">{existente.error.message}</p>
  return <Formulario venta={id ? existente.data : undefined} />
}

function Formulario({ venta }: { venta?: Venta }) {
  const [, navegar] = useLocation()
  const guardar = useGuardarVenta()
  const [cliente, setCliente] = useState<Cliente | null>(venta?.cliente ?? null)
  const [lineas, setLineas] = useState<LineaForm[]>(() => (venta ? desdeVenta(venta) : []))
  const [notas, setNotas] = useState(venta?.notas ?? '')
  const [entrega, setEntrega] = useState(venta?.fecha_entrega ?? '')
  const [buscando, setBuscando] = useState(false)
  const [error, setError] = useState('')

  useEffect(() => { setError('') }, [cliente, lineas])

  const total = useMemo(
    () => lineas.reduce((t, l) => t + (numSimple(l.cantidad) || 0) * (num(l.precio) || 0), 0),
    [lineas],
  )

  const cambiar = (clave: number, cambios: Partial<LineaForm>) =>
    setLineas((ls) => ls.map((l) => (l.clave === clave ? { ...l, ...cambios } : l)))

  const agregarProducto = (p: ProductoConStock) => {
    setBuscando(false)
    setLineas((ls) => [...ls, {
      clave: siguiente++, tipo: 'producto', producto_id: p.id, descripcion: p.nombre, unidad: p.unidad,
      ancho: '', alto: '', prof: '', cantidad: '1', precio: String(p.precio_venta).replace('.', ','),
    }])
  }
  const agregarMueble = () =>
    setLineas((ls) => [...ls, {
      clave: siguiente++, tipo: 'mueble', descripcion: '', unidad: 'unidad', ancho: '', alto: '', prof: '', cantidad: '1', precio: '',
    }])

  const enviar = () => {
    if (!cliente) return setError('Elegí el cliente.')
    if (lineas.length === 0) return setError('Agregá al menos un producto o mueble.')
    guardar.mutate({
      id: venta?.id,
      datos: {
        cliente_id: cliente.id,
        notas,
        fecha_entrega: entrega || null,
        lineas: lineas.map((l) => ({
          tipo: l.tipo,
          producto_id: l.producto_id,
          descripcion: l.descripcion,
          ancho_mm: l.ancho ? Number(l.ancho) : undefined,
          alto_mm: l.alto ? Number(l.alto) : undefined,
          profundidad_mm: l.prof ? Number(l.prof) : undefined,
          cantidad: numSimple(l.cantidad),
          precio_unitario: l.precio === '' ? undefined : num(l.precio),
        })),
      },
    }, { onSuccess: (v) => navegar(`/ventas/${v.id}`) })
  }

  return (
    <div className="mx-auto max-w-4xl space-y-5">
      <div className="flex items-center gap-2">
        <Button variant="ghost" size="icon" asChild aria-label="Volver">
          <Link href={venta ? `/ventas/${venta.id}` : '/ventas'}><ArrowLeft /></Link>
        </Button>
        <h2 className="text-2xl font-semibold tracking-tight">{venta ? `Editar presupuesto N° ${venta.numero}` : 'Nuevo presupuesto'}</h2>
      </div>

      <Card>
        <CardHeader><CardTitle>Cliente</CardTitle></CardHeader>
        <CardContent><SelectorCliente valor={cliente} onChange={setCliente} /></CardContent>
      </Card>

      <Card>
        <CardHeader className="flex flex-row flex-wrap items-center justify-between gap-2">
          <CardTitle>Detalle</CardTitle>
          <div className="flex gap-2">
            <Button type="button" variant="outline" size="sm" onClick={() => setBuscando(true)}><Package /> Producto</Button>
            <Button type="button" variant="outline" size="sm" onClick={agregarMueble}><Hammer /> Mueble a medida</Button>
          </div>
        </CardHeader>
        <CardContent className="space-y-3">
          {lineas.length === 0 && (
            <div className="rounded-lg border border-dashed py-10 text-center text-sm text-muted-foreground">
              Agregá placas, cantos y herrajes del stock, o un mueble a medida.
            </div>
          )}
          {lineas.map((l, i) => {
            const subtotal = (numSimple(l.cantidad) || 0) * (num(l.precio) || 0)
            return (
              <div key={l.clave} className="grid gap-2 rounded-lg border p-3 sm:grid-cols-[1fr_6rem_8rem_7rem_auto] sm:items-end">
                <div className="grid gap-1.5">
                  <span className="text-xs text-muted-foreground">
                    {l.tipo === 'mueble' ? 'Mueble a medida' : 'Producto del stock'}
                  </span>
                  <Input value={l.descripcion} aria-label={`Descripción del renglón ${i + 1}`}
                    placeholder={l.tipo === 'mueble' ? 'Ej: Bajo mesada 2 puertas' : ''}
                    onChange={(e) => cambiar(l.clave, { descripcion: e.target.value })} />
                  {l.tipo === 'mueble' && (
                    <div className="grid grid-cols-3 gap-2">
                      <Input value={l.ancho} inputMode="numeric" placeholder="Ancho mm" aria-label="Ancho en mm"
                        onChange={(e) => cambiar(l.clave, { ancho: e.target.value.replace(/\D/g, '') })} />
                      <Input value={l.alto} inputMode="numeric" placeholder="Alto mm" aria-label="Alto en mm"
                        onChange={(e) => cambiar(l.clave, { alto: e.target.value.replace(/\D/g, '') })} />
                      <Input value={l.prof} inputMode="numeric" placeholder="Prof. mm" aria-label="Profundidad en mm"
                        onChange={(e) => cambiar(l.clave, { prof: e.target.value.replace(/\D/g, '') })} />
                    </div>
                  )}
                </div>
                <label className="grid gap-1.5 text-xs text-muted-foreground">
                  Cantidad {l.tipo === 'producto' && `(${unidadCorta[l.unidad] ?? l.unidad})`}
                  <Input value={l.cantidad} inputMode="decimal" className="text-right" aria-label={`Cantidad del renglón ${i + 1}`}
                    onChange={(e) => cambiar(l.clave, { cantidad: e.target.value })} />
                </label>
                <label className="grid gap-1.5 text-xs text-muted-foreground">
                  Precio unitario
                  <Input value={l.precio} inputMode="decimal" className="text-right" aria-label={`Precio del renglón ${i + 1}`}
                    onChange={(e) => cambiar(l.clave, { precio: e.target.value })} />
                </label>
                <div className="flex items-center justify-between sm:block sm:pb-2 sm:text-right">
                  <span className="text-xs text-muted-foreground sm:hidden">Subtotal</span>
                  <span className="font-medium tabular-nums">{fmtPesos(subtotal)}</span>
                </div>
                <Button type="button" variant="ghost" size="icon" aria-label={`Quitar renglón ${i + 1}`}
                  className="justify-self-end text-muted-foreground"
                  onClick={() => setLineas((ls) => ls.filter((x) => x.clave !== l.clave))}><Trash2 /></Button>
              </div>
            )
          })}
          {lineas.length > 0 && (
            <div className="flex items-center justify-end gap-4 pt-2 text-lg">
              <span className="text-muted-foreground">Total</span>
              <span className="font-semibold tabular-nums">{fmtPesos(total)}</span>
            </div>
          )}
        </CardContent>
      </Card>

      <Card>
        <CardHeader><CardTitle>Entrega y notas</CardTitle></CardHeader>
        <CardContent className="grid gap-4 sm:grid-cols-[12rem_1fr]">
          <label className="grid gap-1.5 text-sm font-medium">
            Fecha de entrega
            <Input type="date" value={entrega} onChange={(e) => setEntrega(e.target.value)} />
          </label>
          <label className="grid gap-1.5 text-sm font-medium">
            Notas (salen en la boleta)
            <Textarea value={notas} onChange={(e) => setNotas(e.target.value)} rows={2} />
          </label>
        </CardContent>
      </Card>

      {(error || guardar.error) && <p className="text-sm text-destructive">{error || guardar.error?.message}</p>}
      <div className="flex justify-end gap-2 pb-6">
        <Button variant="outline" asChild><Link href={venta ? `/ventas/${venta.id}` : '/ventas'}>Cancelar</Link></Button>
        <Button onClick={enviar} disabled={guardar.isPending}>
          {guardar.isPending ? <Loader2 className="animate-spin" /> : <Plus />} {venta ? 'Guardar cambios' : 'Guardar presupuesto'}
        </Button>
      </div>

      {buscando && <BuscarProducto onElegir={agregarProducto} onClose={() => setBuscando(false)} />}
    </div>
  )
}
