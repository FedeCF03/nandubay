import { useState } from 'react'
import { Link, useParams } from 'wouter'
import {
  ArrowLeft, Ban, CheckCircle2, CircleDollarSign, ClipboardCheck, MapPin, MoreHorizontal, Pencil, Printer, Scissors, Truck, Undo2,
} from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import {
  DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Skeleton } from '@/components/ui/skeleton'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import { usePermiso } from '@/auth/sesion'
import { cn } from '@/lib/utils'
import { fmtNumero, fmtPesos, unidadCorta } from '@/stock/api'
import {
  fmtDia, fmtFechaHora, medidas, nombreEstado, nombrePago, numeroVenta, pendiente, useAccionVenta, useVenta,
  type Estado, type Venta as TVenta,
} from '@/ventas/api'
import { DialogoCancelar, DialogoConfirmar, DialogoPago, DialogoRetiro } from '@/ventas/Dialogos'
import { DespieceVenta } from '@/presupuestos/Despiece'
import { EstadoBadge } from './Ventas'

type Dialogo = 'confirmar' | 'pago' | 'retiro' | 'cancelar' | null

function Fila({ label, children, fuerte }: { label: string; children: React.ReactNode; fuerte?: boolean }) {
  return (
    <div className={cn('flex items-center justify-between gap-2', fuerte && 'text-base font-semibold')}>
      <span className={cn(!fuerte && 'text-muted-foreground')}>{label}</span>
      <span className="tabular-nums">{children}</span>
    </div>
  )
}

function Aviso({ v }: { v: TVenta }) {
  if (v.estado === 'presupuesto' || v.estado === 'cancelado' || !v.retira) return null
  if (v.estado === 'entregado') {
    return (
      <div className="flex items-center gap-2 rounded-lg bg-primary/10 p-3 text-sm">
        <CheckCircle2 className="size-5 shrink-0 text-primary" /> Entregada completa en {v.retira.nombre}.
      </div>
    )
  }
  return (
    <div className={cn('flex items-start gap-2 rounded-lg p-3 text-sm',
      v.retira_aqui ? 'bg-emerald-500/10 text-emerald-900 dark:text-emerald-200' : 'bg-sky-500/10 text-sky-900 dark:text-sky-200')}>
      <MapPin className="mt-0.5 size-4 shrink-0" />
      <div>
        Se retira en <b>{v.retira.nombre}</b> ({v.deposito}){v.retira.direccion && `, ${v.retira.direccion}`}.
        {v.retira_aqui ? (v.pagada ? ' Podés entregarla desde esta sucursal.' : ` Para entregar tiene que estar paga: faltan ${fmtPesos(v.saldo)}.`)
          : ' Para entregarla, entrá a esa sucursal.'}
      </div>
    </div>
  )
}

export default function Venta() {
  const { id } = useParams<{ id: string }>()
  const venta = useVenta(Number(id))
  const accion = useAccionVenta(Number(id))
  const [dialogo, setDialogo] = useState<Dialogo>(null)
  const edita = usePermiso('pedidos.editar')
  const confirma = usePermiso('pedidos.confirmar')
  const cobra = usePermiso('pagos.registrar')
  const entrega = usePermiso('retiros.registrar')
  const produccion = usePermiso('pedidos.estado')

  if (venta.isPending) return <div className="space-y-4"><Skeleton className="h-24" /><Skeleton className="h-72" /></div>
  if (venta.isError) return <p className="text-destructive">{venta.error.message}</p>
  const v = venta.data
  const esPresupuesto = v.estado === 'presupuesto'
  const confirmada = v.estados_produccion.includes(v.estado)
  const activa = v.estado !== 'cancelado'
  const cancela = confirma && activa && v.estado !== 'entregado'
  const devuelve = cobra && activa && v.pagado > 0
  const muebles = v.tipo === 'muebles'
  const base = muebles ? '/presupuestos' : '/ventas'

  return (
    <div className="mx-auto max-w-5xl space-y-5">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div className="flex items-center gap-2">
          <Button variant="ghost" size="icon" asChild aria-label="Volver"><Link href={base}><ArrowLeft /></Link></Button>
          <div>
            <div className="flex items-center gap-2">
              <h2 className="text-2xl font-semibold tracking-tight">{esPresupuesto ? 'Presupuesto' : 'Venta'} N° {numeroVenta(v.numero)}</h2>
              <EstadoBadge estado={v.estado} />
            </div>
            <p className="text-sm text-muted-foreground">
              {fmtFechaHora(v.creado_en)} · {v.sucursal.nombre}{v.vendedor && ` · ${v.vendedor}`}
            </p>
          </div>
        </div>
        <div className="flex flex-wrap gap-2">
          <Button variant="outline" asChild>
            <Link href={`/ventas/${v.id}/boleta`}><Printer /> {esPresupuesto ? 'Imprimir presupuesto' : 'Imprimir boleta'}</Link>
          </Button>
          {muebles && (
            <Button variant="outline" asChild><Link href={`/presupuestos/${v.id}/cortes`}><Scissors /> Plano de cortes</Link></Button>
          )}
          {muebles && confirmada && (
            <Button variant="outline" asChild><Link href={`/cortes-optimizados?pedido=${v.id}`}><ClipboardCheck /> Cortes</Link></Button>
          )}
          {esPresupuesto && edita && (
            <Button variant="outline" asChild><Link href={`${base}/${v.id}/editar`}><Pencil /> Editar</Link></Button>
          )}
          {esPresupuesto && confirma && <Button onClick={() => setDialogo('confirmar')}><CheckCircle2 /> Confirmar venta</Button>}
          {activa && cobra && v.saldo > 0 && (
            <Button variant={esPresupuesto ? 'outline' : 'default'} onClick={() => setDialogo('pago')}><CircleDollarSign /> Registrar pago</Button>
          )}
          {confirmada && entrega && v.retira_aqui && !v.todo_retirado && (
            <Button onClick={() => setDialogo('retiro')} disabled={!v.pagada}><Truck /> Entregar</Button>
          )}
          {(cancela || devuelve) && (
            <DropdownMenu>
              <DropdownMenuTrigger asChild>
                <Button variant="ghost" size="icon" aria-label="Más acciones"><MoreHorizontal /></Button>
              </DropdownMenuTrigger>
              <DropdownMenuContent align="end">
                {devuelve && <DropdownMenuItem onSelect={() => setDialogo('pago')}><Undo2 /> Registrar devolución</DropdownMenuItem>}
                {cancela && <DropdownMenuItem variant="destructive" onSelect={() => setDialogo('cancelar')}><Ban /> Cancelar venta</DropdownMenuItem>}
              </DropdownMenuContent>
            </DropdownMenu>
          )}
        </div>
      </div>

      <Aviso v={v} />
      {accion.error && <p className="text-sm text-destructive">{accion.error.message}</p>}

      <div className="grid gap-5 lg:grid-cols-[1fr_20rem]">
        <Card className="py-0">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead className="pl-4">Detalle</TableHead>
                <TableHead className="text-right">Cant.</TableHead>
                {confirmada || v.estado === 'entregado' ? <TableHead className="text-right">Retirado</TableHead> : null}
                <TableHead className="hidden text-right sm:table-cell">Precio</TableHead>
                <TableHead className="pr-4 text-right">Subtotal</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {v.lineas.map((l) => (
                <TableRow key={`${l.tipo}-${l.id}`}>
                  <TableCell className="pl-4 whitespace-normal">
                    <div className="font-medium">{l.descripcion}</div>
                    <div className="text-xs text-muted-foreground">{l.tipo === 'mueble' ? ['Mueble a medida', medidas(l)].filter(Boolean).join(' · ') : 'Del stock'}</div>
                  </TableCell>
                  <TableCell className="text-right tabular-nums">{fmtNumero(l.cantidad)} <span className="text-xs text-muted-foreground">{unidadCorta[l.unidad]}</span></TableCell>
                  {confirmada || v.estado === 'entregado' ? (
                    <TableCell className={cn('text-right tabular-nums', pendiente(l) > 0 ? 'text-amber-700 dark:text-amber-400' : 'text-emerald-700 dark:text-emerald-400')}>
                      {fmtNumero(l.retirado)}
                    </TableCell>
                  ) : null}
                  <TableCell className="hidden text-right tabular-nums sm:table-cell">{fmtPesos(l.precio_unitario)}</TableCell>
                  <TableCell className="pr-4 text-right tabular-nums">{fmtPesos(l.cantidad * l.precio_unitario)}</TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </Card>

        <div className="space-y-5">
          <Card className="gap-3">
            <CardHeader><CardTitle>Importes</CardTitle></CardHeader>
            <CardContent className="space-y-1.5 text-sm">
              <Fila label="Total" fuerte>{fmtPesos(v.total)}</Fila>
              <Fila label="Pagado">{fmtPesos(v.pagado)}</Fila>
              <Fila label="Saldo">
                <span className={cn('font-medium', v.saldo > 0 ? 'text-amber-700 dark:text-amber-400' : 'text-emerald-700 dark:text-emerald-400')}>
                  {v.saldo > 0 ? fmtPesos(v.saldo) : 'Pagada'}
                </span>
              </Fila>
            </CardContent>
          </Card>

          <Card className="gap-3">
            <CardHeader><CardTitle>Cliente</CardTitle></CardHeader>
            <CardContent className="space-y-0.5 text-sm">
              <div className="font-medium">{v.cliente.nombre}</div>
              {v.cliente.documento && <div className="text-muted-foreground">DNI/CUIT {v.cliente.documento}</div>}
              {v.cliente.telefono && <div><a className="text-primary hover:underline" href={`tel:${v.cliente.telefono}`}>{v.cliente.telefono}</a></div>}
              {v.cliente.direccion && <div className="text-muted-foreground">{v.cliente.direccion}</div>}
              {v.fecha_entrega && <div className="pt-2">Entrega: <b>{fmtDia(v.fecha_entrega)}</b></div>}
              {v.notas && <p className="pt-2 whitespace-pre-line text-muted-foreground">{v.notas}</p>}
            </CardContent>
          </Card>

          {confirmada && produccion && (
            <Card className="gap-3">
              <CardHeader><CardTitle>Producción</CardTitle></CardHeader>
              <CardContent>
                <Select value={v.estado} onValueChange={(e) => accion.mutate({ accion: 'estado', estado: e as Estado })}>
                  <SelectTrigger className="w-full" aria-label="Estado de producción"><SelectValue /></SelectTrigger>
                  <SelectContent>
                    {v.estados_produccion.map((e) => <SelectItem key={e} value={e}>{nombreEstado[e]}</SelectItem>)}
                  </SelectContent>
                </Select>
              </CardContent>
            </Card>
          )}
        </div>
      </div>

      <div className="grid gap-5 md:grid-cols-2">
        <Card className="gap-3">
          <CardHeader><CardTitle>Pagos</CardTitle></CardHeader>
          <CardContent className="text-sm">
            {v.pagos.length === 0 ? <p className="text-muted-foreground">Todavía no hay pagos.</p> : (
              <ul className="divide-y">
                {v.pagos.map((p) => (
                  <li key={p.id} className="flex items-start justify-between gap-2 py-2">
                    <div>
                      <div className="font-medium">{nombrePago[p.tipo]} · {p.medio}</div>
                      <div className="text-xs text-muted-foreground">{fmtFechaHora(p.fecha)} · {p.usuario}{p.nota && ` · ${p.nota}`}</div>
                    </div>
                    <span className={cn('tabular-nums', p.tipo === 'devolucion' && 'text-destructive')}>
                      {p.tipo === 'devolucion' ? '−' : ''}{fmtPesos(p.monto)}
                    </span>
                  </li>
                ))}
              </ul>
            )}
          </CardContent>
        </Card>

        <Card className="gap-3">
          <CardHeader><CardTitle>Retiros</CardTitle></CardHeader>
          <CardContent className="text-sm">
            {v.retiros.length === 0 ? <p className="text-muted-foreground">El cliente todavía no retiró nada.</p> : (
              <ul className="divide-y">
                {v.retiros.map((r) => (
                  <li key={r.id} className="py-2">
                    <div className="flex justify-between gap-2">
                      <span className="font-medium">{r.sucursal}</span>
                      <span className="text-xs text-muted-foreground">{fmtFechaHora(r.fecha)} · {r.usuario}</span>
                    </div>
                    <ul className="text-muted-foreground">
                      {r.items.map((i) => <li key={`${i.tipo}-${i.linea_id}`}>{fmtNumero(i.cantidad)} × {i.descripcion}</li>)}
                    </ul>
                    {r.nota && <p className="text-xs text-muted-foreground">{r.nota}</p>}
                  </li>
                ))}
              </ul>
            )}
          </CardContent>
        </Card>
      </div>

      {muebles && <DespieceVenta id={v.id} />}

      {dialogo === 'confirmar' && <DialogoConfirmar venta={v} onClose={() => setDialogo(null)} />}
      {dialogo === 'pago' && <DialogoPago venta={v} onClose={() => setDialogo(null)} />}
      {dialogo === 'retiro' && <DialogoRetiro venta={v} onClose={() => setDialogo(null)} />}
      {dialogo === 'cancelar' && <DialogoCancelar venta={v} onClose={() => setDialogo(null)} />}
    </div>
  )
}
