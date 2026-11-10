import { useEffect, useMemo } from 'react'
import { Link, useRoute } from 'wouter'
import { ArrowLeft, Loader2, Printer } from 'lucide-react'
import { renderSVG } from 'uqr'
import { Button } from '@/components/ui/button'
import { fmtNumero, fmtPesos, unidadCorta } from '@/stock/api'
import { fmtDia, fmtFecha, medidas, nombrePago, numeroVenta, pendiente, useVenta, type Venta } from '@/ventas/api'

// Boleta para imprimir en A4. Sirve de comprobante de pago y para retirar la
// mercadería en la sucursal del depósito de donde salió el stock.
export default function Boleta() {
  // Se muestra fuera del menú (sin <Route>), así que lee el id de la ruta acá.
  const [, params] = useRoute('/ventas/:id/boleta')
  const venta = useVenta(Number(params?.id))

  useEffect(() => {
    if (venta.data) document.title = `${venta.data.estado === 'presupuesto' ? 'Presupuesto' : 'Boleta'} ${numeroVenta(venta.data.numero)}`
    return () => { document.title = 'Ñandubay' }
  }, [venta.data])

  if (venta.isPending) {
    return <div className="flex min-h-svh items-center justify-center"><Loader2 className="size-6 animate-spin text-muted-foreground" /></div>
  }
  if (venta.isError) return <p className="p-6 text-destructive">{venta.error.message}</p>
  return (
    <div className="min-h-svh bg-muted/60 print:bg-white">
      <div className="no-imprimir sticky top-0 z-10 flex items-center justify-between gap-2 border-b bg-background/95 px-4 py-2 backdrop-blur">
        <Button variant="ghost" asChild><Link href={`/ventas/${venta.data.id}`}><ArrowLeft /> Volver</Link></Button>
        <Button onClick={() => window.print()}><Printer /> Imprimir</Button>
      </div>
      <div className="overflow-x-auto py-6 print:overflow-visible print:py-0">
        <Hoja v={venta.data} />
      </div>
    </div>
  )
}

function Sello({ v }: { v: Venta }) {
  if (v.estado === 'presupuesto') return null
  if (v.estado === 'cancelado') {
    return <div className="sello border-red-700 text-red-700">ANULADA</div>
  }
  if (v.pagada) return <div className="sello border-emerald-700 text-emerald-700">PAGADA</div>
  return (
    <div className="sello border-amber-700 text-amber-700">
      SALDO PENDIENTE
      <div className="text-lg tracking-normal">{fmtPesos(v.saldo)}</div>
    </div>
  )
}

function Hoja({ v }: { v: Venta }) {
  const esPresupuesto = v.estado === 'presupuesto'
  const url = `${window.location.origin}/ventas/${v.id}`
  const qr = useMemo(() => renderSVG(url, { border: 1 }), [url])
  const conRetiros = v.retiros.length > 0
  const queda = v.lineas.filter((l) => pendiente(l) > 0)

  return (
    <article className="boleta mx-auto flex w-[210mm] min-h-[297mm] flex-col gap-5 bg-white p-[14mm] text-[10.5pt] text-neutral-900 shadow-lg print:shadow-none">
      {/* Encabezado */}
      <header className="flex items-start justify-between gap-6 border-b-2 border-neutral-800 pb-4">
        <div className="min-w-0">
          <h1 className="text-[20pt] leading-tight font-bold">{v.carpinteria}</h1>
          <p>Sucursal {v.sucursal.nombre}</p>
          {v.sucursal.direccion && <p className="text-neutral-600">{v.sucursal.direccion}</p>}
          {v.sucursal.telefono && <p className="text-neutral-600">Tel. {v.sucursal.telefono}</p>}
        </div>
        <div className="flex shrink-0 items-start gap-3">
          <div className="flex size-14 items-center justify-center border-2 border-neutral-800 text-[26pt] font-bold leading-none">X</div>
          <div className="text-right">
            <div className="text-[16pt] font-bold tracking-wide">{esPresupuesto ? 'PRESUPUESTO' : 'BOLETA'}</div>
            <div className="font-mono text-[13pt]">N° {numeroVenta(v.numero)}</div>
            <div>Fecha: {fmtFecha(v.confirmado_en ?? v.creado_en)}</div>
            <div className="mt-1 text-[8pt] font-semibold uppercase">Documento no válido como factura</div>
          </div>
        </div>
      </header>

      {/* Cliente */}
      <section className="grid grid-cols-2 gap-x-6 gap-y-0.5 rounded border border-neutral-300 p-3">
        <div><span className="text-neutral-500">Cliente: </span><b>{v.cliente.nombre}</b></div>
        <div><span className="text-neutral-500">DNI/CUIT: </span>{v.cliente.documento || '—'}</div>
        <div><span className="text-neutral-500">Teléfono: </span>{v.cliente.telefono || '—'}</div>
        <div><span className="text-neutral-500">Dirección: </span>{v.cliente.direccion || '—'}</div>
        {v.fecha_entrega && <div><span className="text-neutral-500">Entrega pactada: </span>{fmtDia(v.fecha_entrega)}</div>}
        {v.vendedor && <div><span className="text-neutral-500">Vendedor: </span>{v.vendedor}</div>}
      </section>

      {/* Detalle */}
      <table className="w-full border-collapse">
        <thead>
          <tr className="border-y border-neutral-800 text-left text-[9pt] uppercase">
            <th className="py-1.5 pr-2 text-right">Cant.</th>
            <th className="py-1.5 px-2">Descripción</th>
            {conRetiros && <th className="py-1.5 px-2 text-right">Retirado</th>}
            <th className="py-1.5 px-2 text-right">P. unitario</th>
            <th className="py-1.5 pl-2 text-right">Importe</th>
          </tr>
        </thead>
        <tbody>
          {v.lineas.map((l) => (
            <tr key={`${l.tipo}-${l.id}`} className="border-b border-neutral-200 align-top">
              <td className="py-1.5 pr-2 text-right tabular-nums whitespace-nowrap">{fmtNumero(l.cantidad)} {unidadCorta[l.unidad]}</td>
              <td className="py-1.5 px-2">
                {l.descripcion}
                {l.tipo === 'mueble' && medidas(l) && <span className="text-neutral-500"> ({medidas(l)})</span>}
              </td>
              {conRetiros && <td className="py-1.5 px-2 text-right tabular-nums">{fmtNumero(l.retirado)}</td>}
              <td className="py-1.5 px-2 text-right tabular-nums whitespace-nowrap">{fmtPesos(l.precio_unitario)}</td>
              <td className="py-1.5 pl-2 text-right tabular-nums whitespace-nowrap">{fmtPesos(l.cantidad * l.precio_unitario)}</td>
            </tr>
          ))}
        </tbody>
      </table>

      {/* Totales y sello */}
      <section className="flex items-start justify-between gap-6">
        <div className="min-w-0 flex-1 space-y-2">
          {v.notas && <p className="whitespace-pre-line"><span className="text-neutral-500">Notas: </span>{v.notas}</p>}
          {v.pagos.length > 0 && (
            <div className="text-[9pt] text-neutral-600">
              {v.pagos.map((p) => (
                <div key={p.id}>{fmtFecha(p.fecha)} · {nombrePago[p.tipo]} {p.medio}: {p.tipo === 'devolucion' ? '−' : ''}{fmtPesos(p.monto)}</div>
              ))}
            </div>
          )}
        </div>
        <div className="relative w-64 shrink-0">
          <div className="space-y-0.5">
            <div className="flex justify-between border-t-2 border-neutral-800 pt-1 text-[13pt] font-bold">
              <span>TOTAL</span><span className="tabular-nums">{fmtPesos(v.total)}</span>
            </div>
            {!esPresupuesto && (
              <>
                <div className="flex justify-between"><span>Pagado</span><span className="tabular-nums">{fmtPesos(v.pagado)}</span></div>
                <div className="flex justify-between font-semibold"><span>Saldo</span><span className="tabular-nums">{fmtPesos(v.saldo)}</span></div>
              </>
            )}
          </div>
        </div>
      </section>
      <div className="flex justify-center"><Sello v={v} /></div>

      {/* Retiro */}
      {v.retira && v.estado !== 'cancelado' ? (
        <section className="mt-auto flex items-stretch gap-4 rounded border-2 border-dashed border-neutral-800 p-4">
          <div className="size-32 shrink-0" dangerouslySetInnerHTML={{ __html: qr }} aria-label="Código QR de la venta" />
          <div className="flex min-w-0 flex-1 flex-col justify-between gap-2">
            <div>
              <div className="text-[9pt] font-semibold uppercase text-neutral-500">Retira en</div>
              <div className="text-[15pt] font-bold leading-tight">Sucursal {v.retira.nombre}</div>
              <div>{v.deposito}{v.retira.direccion && ` · ${v.retira.direccion}`}{v.retira.telefono && ` · Tel. ${v.retira.telefono}`}</div>
            </div>
            {v.estado === 'entregado' ? (
              <p className="font-semibold">Mercadería entregada en su totalidad.</p>
            ) : (
              <div className="text-[9pt] text-neutral-700">
                {conRetiros && queda.length > 0 && (
                  <p className="font-semibold text-neutral-900">
                    Queda por retirar: {queda.map((l) => `${fmtNumero(pendiente(l))} ${l.descripcion}`).join(', ')}.
                  </p>
                )}
                <p>Presentá esta boleta para retirar. Se puede retirar en partes; en cada retiro se registra lo entregado.
                  {!v.pagada && ' Para retirar la venta tiene que estar paga.'}</p>
              </div>
            )}
            <div className="font-mono text-[9pt] text-neutral-500">N° {numeroVenta(v.numero)}</div>
          </div>
        </section>
      ) : esPresupuesto ? (
        <p className="mt-auto rounded border border-neutral-300 p-3 text-[9pt] text-neutral-600">
          Presupuesto sujeto a disponibilidad de stock al momento de confirmar. Los precios pueden cambiar sin previo aviso.
        </p>
      ) : <div className="mt-auto" />}

      <footer className="border-t border-neutral-300 pt-2 text-center text-[8pt] text-neutral-500">
        Documento no válido como factura · Comprobante interno de {v.carpinteria}
      </footer>
    </article>
  )
}
