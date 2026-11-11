import { useMemo } from 'react'
import { Layers, Package, Ruler } from 'lucide-react'
import { PlacaSVG } from '@/cortes/PlacaSVG'
import type { Pieza } from '@/cortes/tipos'
import { fmtNumero, fmtPesos, unidadCorta, useProductos } from '@/stock/api'
import { cuantasPlacas, type Materiales, type Mueble } from './api'

function Renglon({ nombre, detalle, costo }: { nombre: string; detalle: string; costo: number }) {
  return (
    <li className="flex items-start justify-between gap-3 py-1.5">
      <div className="min-w-0">
        <div className="truncate font-medium">{nombre}</div>
        <div className="text-xs text-muted-foreground">{detalle}</div>
      </div>
      <span className="shrink-0 tabular-nums">{fmtPesos(costo)}</span>
    </li>
  )
}

// ResumenMateriales lista lo que hay que usar o comprar para los muebles.
export function ResumenMateriales({ mat }: { mat: Materiales }) {
  const grupo = (titulo: string, icono: React.ReactNode, hijos: React.ReactNode) => (
    <div>
      <div className="flex items-center gap-1.5 text-xs font-medium tracking-wide text-muted-foreground uppercase">{icono} {titulo}</div>
      <ul className="divide-y text-sm">{hijos}</ul>
    </div>
  )
  return (
    <div className="space-y-4">
      {mat.placas.length > 0 && grupo('Placas', <Layers className="size-3.5" />, mat.placas.map((p) => (
        <Renglon key={p.producto_id} nombre={p.nombre} costo={p.costo}
          detalle={[cuantasPlacas(p.cantidad, p.retazos), `aprovechamiento ${Math.round(p.aprovechamiento)} %`].join(' · ')} />
      )))}
      {mat.cantos.length > 0 && grupo('Canto', <Ruler className="size-3.5" />, mat.cantos.map((c) => (
        <Renglon key={c.producto_id} nombre={c.nombre} costo={c.costo} detalle={`${fmtNumero(c.cantidad)} m`} />
      )))}
      {mat.insumos.length > 0 && grupo('Herrajes e insumos', <Package className="size-3.5" />, mat.insumos.map((c) => (
        <Renglon key={c.producto_id} nombre={c.nombre} costo={c.costo}
          detalle={`${fmtNumero(c.cantidad)} ${unidadCorta[c.unidad] ?? c.unidad}`} />
      )))}
      <div className="flex items-center justify-between border-t pt-2 font-semibold">
        <span>Costo de materiales</span>
        <span className="tabular-nums">{fmtPesos(mat.costo)}</span>
      </div>
    </div>
  )
}

// piezasDePlan arma las piezas con el mismo id que les pone el backend
// (mueble-pieza), para que el dibujo sepa los cantos y el color de cada una.
function piezasDePlan(muebles: Mueble[]): Pieza[] {
  return muebles.flatMap((m, i) => m.piezas.map((p, j) => ({
    id: `${i}-${j}`, etiqueta: p.nombre, largo: p.largo, ancho: p.ancho,
    cantidad: p.cantidad * m.cantidad, respeta_veta: p.respeta_veta, cantos: p.cantos,
  })))
}

// PlanesDeCorte dibuja cómo se corta cada placa con las piezas de todos los muebles.
export function PlanesDeCorte({ mat, muebles }: { mat: Materiales; muebles: Mueble[] }) {
  const placas = useProductos({ tipo: 'placa', buscar: '' })
  const piezas = useMemo(() => piezasDePlan(muebles), [muebles])
  const veta = new Map((placas.data ?? []).map((p) => [p.id, p.tiene_veta]))
  return (
    <div className="space-y-6">
      {mat.placas.map((pc) => (
        <section key={pc.producto_id} className="space-y-3 break-inside-avoid">
          <h3 className="font-semibold">
            {pc.nombre} <span className="font-normal text-muted-foreground">· {cuantasPlacas(pc.cantidad, pc.retazos)}</span>
          </h3>
          <div className="grid gap-4 md:grid-cols-2">
            {pc.plan.placas.map((pl) => (
              <figure key={pl.indice} className="space-y-1 break-inside-avoid">
                <figcaption className="flex justify-between text-sm text-muted-foreground">
                  <span>{pl.retazo_id ? `Retazo #${pl.retazo_id} (${pl.largo} × ${pl.ancho})` : `Placa ${pl.indice}`} · {pl.piezas.length} piezas</span>
                  <span>{Math.round(pl.aprovechamiento)} %</span>
                </figcaption>
                <PlacaSVG placa={pl} piezas={piezas} tieneVeta={veta.get(pc.producto_id) ?? false} />
              </figure>
            ))}
          </div>
        </section>
      ))}
    </div>
  )
}
