import { useEffect, useMemo } from 'react'
import { Link, useRoute } from 'wouter'
import { ArrowLeft, Loader2, Printer } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { PlacaSVG } from '@/cortes/PlacaSVG'
import type { Cantos, Pieza } from '@/cortes/tipos'
import { fmtNumero, unidadCorta, useProductos } from '@/stock/api'
import { fmtDia, medidas, numeroVenta } from '@/ventas/api'
import { cuantasPlacas, tieneCanto, usePresupuesto, type PresupuestoMuebles } from '@/presupuestos/api'

const lados = (c: Cantos) => (['l1', 'l2', 'a1', 'a2'] as const).filter((k) => c[k]).map((k) => k.toUpperCase()).join(' ')

// Plano de cortes para el taller: una hoja con el despiece y los materiales,
// y una hoja apaisada por cada placa a cortar.
export default function PlanoCortes() {
  // Se muestra fuera del menú (sin <Route>), así que lee el id de la ruta acá.
  const [, params] = useRoute('/presupuestos/:id/cortes')
  const id = Number(params?.id)
  const p = usePresupuesto(id)

  useEffect(() => {
    if (p.data) document.title = `Cortes ${numeroVenta(p.data.venta.numero)}`
    return () => { document.title = 'Ñandubay' }
  }, [p.data])

  if (p.isPending) {
    return <div className="flex min-h-svh items-center justify-center"><Loader2 className="size-6 animate-spin text-muted-foreground" /></div>
  }
  if (p.isError) return <p className="p-6 text-destructive">{p.error.message}</p>
  return (
    <div className="min-h-svh bg-muted/60 print:bg-white">
      <div className="no-imprimir sticky top-0 z-10 flex items-center justify-between gap-2 border-b bg-background/95 px-4 py-2 backdrop-blur">
        <Button variant="ghost" asChild><Link href={`/presupuestos/${id}`}><ArrowLeft /> Volver</Link></Button>
        <Button onClick={() => window.print()}><Printer /> Imprimir</Button>
      </div>
      <div className="space-y-6 overflow-x-auto py-6 print:space-y-0 print:overflow-visible print:py-0">
        <Hojas p={p.data} />
      </div>
    </div>
  )
}

function Hojas({ p }: { p: PresupuestoMuebles }) {
  const productos = useProductos({ tipo: '', buscar: '' }).data ?? []
  const nombre = new Map(productos.map((x) => [x.id, x]))
  const { venta: v, muebles, materiales: mat } = p
  // Mismas ids que pone el backend (mueble-pieza), para los cantos y colores.
  const piezas: Pieza[] = useMemo(() => muebles.flatMap((m, i) => m.piezas.map((pz, j) => ({
    id: `${i}-${j}`, etiqueta: pz.nombre, largo: pz.largo, ancho: pz.ancho,
    cantidad: pz.cantidad * m.cantidad, respeta_veta: pz.respeta_veta, cantos: pz.cantos,
  }))), [muebles])
  const porId = new Map(piezas.map((x) => [x.id, x]))
  const titulo = `Presupuesto N° ${numeroVenta(v.numero)} · ${v.cliente.nombre}`

  return (
    <>
      <article className="hoja-cortes mx-auto flex w-[210mm] min-h-[297mm] flex-col gap-5 bg-white p-[14mm] text-[10pt] text-neutral-900 shadow-lg print:shadow-none">
        <header className="flex items-start justify-between gap-4 border-b-2 border-neutral-900 pb-3">
          <div>
            <h1 className="text-xl font-bold">Plano de cortes</h1>
            <p>{titulo}</p>
          </div>
          <div className="text-right text-sm">
            <div>{v.carpinteria}</div>
            {v.fecha_entrega && <div>Entrega: <b>{fmtDia(v.fecha_entrega)}</b></div>}
          </div>
        </header>

        {mat && (
          <section>
            <h2 className="mb-1 font-bold uppercase">Materiales</h2>
            <table className="w-full">
              <tbody>
                {mat.placas.map((x) => (
                  <tr key={x.producto_id} className="border-b border-neutral-300">
                    <td className="py-1">{x.nombre}</td>
                    <td className="py-1 text-right font-semibold">
                      {cuantasPlacas(x.cantidad, x.retazos)}
                      {x.retazos > 0 && <span className="font-normal text-neutral-600"> ({x.plan.placas.filter((pl) => pl.retazo_id).map((pl) => `#${pl.retazo_id}`).join(', ')})</span>}
                    </td>
                  </tr>
                ))}
                {mat.cantos.map((x) => (
                  <tr key={x.producto_id} className="border-b border-neutral-300">
                    <td className="py-1">{x.nombre}</td>
                    <td className="py-1 text-right font-semibold">{fmtNumero(x.cantidad)} m</td>
                  </tr>
                ))}
                {mat.insumos.map((x) => (
                  <tr key={x.producto_id} className="border-b border-neutral-300">
                    <td className="py-1">{x.nombre}</td>
                    <td className="py-1 text-right font-semibold">{fmtNumero(x.cantidad)} {unidadCorta[x.unidad] ?? x.unidad}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </section>
        )}

        {muebles.map((m, i) => (
          <section key={i} className="break-inside-avoid">
            <h2 className="font-bold">{m.cantidad > 1 && `${m.cantidad} × `}{m.nombre}
              {medidas(m) && <span className="font-normal text-neutral-600"> · {medidas(m)}</span>}</h2>
            {m.piezas.length > 0 && (
              <table className="mt-1 w-full">
                <thead>
                  <tr className="border-b border-neutral-900 text-left text-[9pt] uppercase">
                    <th className="py-1">Pieza</th>
                    <th className="py-1">Placa</th>
                    <th className="py-1 text-right">Largo × ancho</th>
                    <th className="py-1 text-right">Cant.</th>
                    <th className="py-1 pl-3">Canto</th>
                  </tr>
                </thead>
                <tbody>
                  {m.piezas.map((pz, j) => (
                    <tr key={j} className="border-b border-neutral-300">
                      <td className="py-1">{pz.nombre || `Pieza ${j + 1}`}</td>
                      <td className="py-1">{nombre.get(pz.placa_id)?.nombre}</td>
                      <td className="py-1 text-right tabular-nums">{pz.largo} × {pz.ancho}</td>
                      <td className="py-1 text-right tabular-nums">
                        {pz.cantidad * m.cantidad}
                        {m.cantidad > 1 && <span className="text-neutral-500"> ({pz.cantidad}×{m.cantidad})</span>}
                      </td>
                      <td className="py-1 pl-3">{tieneCanto(pz.cantos) ? lados(pz.cantos) : '—'}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            )}
          </section>
        ))}
        <p className="mt-auto text-[8.5pt] text-neutral-600">
          Medidas en mm. L1 y L2 son los lados largos; A1 y A2, los anchos. Sierra de 3 mm. En el dibujo, el canto va marcado en rojo.
        </p>
      </article>

      {mat?.placas.flatMap((pc) => pc.plan.placas.map((pl) => (
        <article key={`${pc.producto_id}-${pl.indice}`}
          className="hoja-cortes hoja-apaisada mx-auto flex h-[210mm] w-[297mm] flex-col overflow-hidden gap-3 bg-white p-[10mm] text-[9.5pt] text-neutral-900 shadow-lg print:shadow-none">
          <header className="flex items-baseline justify-between gap-4 border-b-2 border-neutral-900 pb-2">
            <h2 className="text-base font-bold">
              {pc.nombre} · {pl.retazo_id ? `retazo #${pl.retazo_id} (${pl.largo} × ${pl.ancho} mm)` : `placa entera`} · {pl.indice} de {pc.plan.placas.length}
            </h2>
            <span>{titulo} · {pl.piezas.length} piezas · {Math.round(pl.aprovechamiento)} %</span>
          </header>
          <div className="min-h-0 flex-1 [&_svg]:h-full [&_svg]:w-full">
            <PlacaSVG placa={pl} piezas={piezas} tieneVeta={nombre.get(pc.producto_id)?.tiene_veta ?? false} />
          </div>
          <ol className="columns-3 gap-6 text-[8.5pt]">
            {pl.piezas.map((c, k) => {
              const pz = porId.get(c.pieza_id)
              return (
                <li key={k} className="break-inside-avoid">
                  ☐ {c.etiqueta}: {c.largo} × {c.ancho}{c.rotada ? ' (girada)' : ''}{pz && tieneCanto(pz.cantos) ? ` · canto ${lados(pz.cantos)}` : ''}
                </li>
              )
            })}
          </ol>
          {(pl.sobrantes ?? []).length > 0 && (
            <p className="text-[8.5pt]">
              <b>Guardar como retazo:</b> {(pl.sobrantes ?? []).map((r) => `${r.largo} × ${r.ancho}`).join(' · ')}
            </p>
          )}
        </article>
      )))}
    </>
  )
}
