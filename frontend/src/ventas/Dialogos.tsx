import { useMemo, useState, type FormEvent, type ReactNode } from 'react'
import { AlertTriangle, CheckCircle2, Loader2 } from 'lucide-react'
import { Button } from '@/components/ui/button'
import {
  Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle,
} from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { Textarea } from '@/components/ui/textarea'
import { useUsuario } from '@/auth/sesion'
import { usePresupuesto } from '@/presupuestos/api'
import { cn } from '@/lib/utils'
import { fmtNumero, fmtPesos, unidadCorta, useDepositos, useProductos } from '@/stock/api'
import { mediosPago, pendiente, useAccionVenta, type Pago, type Venta } from './api'

function Marco({ titulo, desc, onClose, onSubmit, children, boton, deshabilitado, pendiente: cargando, error, destructivo }: {
  titulo: string; desc?: ReactNode; onClose: () => void; onSubmit: () => void; children?: ReactNode
  boton: string; deshabilitado?: boolean; pendiente: boolean; error?: Error | null; destructivo?: boolean
}) {
  const enviar = (e: FormEvent) => { e.preventDefault(); onSubmit() }
  return (
    <Dialog open onOpenChange={(o) => !o && onClose()}>
      <DialogContent className="max-h-[90svh] overflow-y-auto">
        <form onSubmit={enviar} className="grid gap-4">
          <DialogHeader>
            <DialogTitle>{titulo}</DialogTitle>
            {desc && <DialogDescription>{desc}</DialogDescription>}
          </DialogHeader>
          {children}
          {error && <p className="text-sm text-destructive">{error.message}</p>}
          <DialogFooter>
            <Button type="button" variant="outline" onClick={onClose}>Volver</Button>
            <Button type="submit" variant={destructivo ? 'destructive' : 'default'} disabled={deshabilitado || cargando}>
              {cargando && <Loader2 className="animate-spin" />} {boton}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}

// DialogoConfirmar elige de qué depósito sale la mercadería. Muestra en cuáles
// alcanza el stock; puede ser de otra sucursal y ahí es donde se retira.
export function DialogoConfirmar({ venta, onClose }: { venta: Venta; onClose: () => void }) {
  const u = useUsuario()
  const accion = useAccionVenta(venta.id)
  const depositos = useDepositos().data ?? []
  const productos = useProductos({ tipo: '', buscar: '' }).data ?? []
  const muebles = venta.tipo === 'muebles'
  const presupuesto = usePresupuesto(muebles ? venta.id : 0)
  const mat = presupuesto.data?.materiales

  // Cuánto hace falta de cada producto: los renglones sueltos y, en un
  // presupuesto de muebles, el canto y los herrajes calculados. Las placas se
  // descuentan después, al confirmar el corte.
  const necesita = useMemo(() => {
    const m = new Map<number, number>()
    const sumar = (id: number, c: number) => m.set(id, (m.get(id) ?? 0) + c)
    for (const l of venta.lineas) if (l.tipo === 'producto' && l.producto_id) sumar(l.producto_id, l.cantidad)
    if (mat) for (const x of [...mat.cantos, ...mat.insumos]) sumar(x.producto_id, x.cantidad)
    return m
  }, [venta.lineas, mat])

  const faltantes = (depositoID: number) => {
    const out: string[] = []
    for (const [pid, cant] of necesita) {
      const p = productos.find((x) => x.id === pid)
      const hay = p?.por_deposito[String(depositoID)] ?? 0
      if (hay < cant) out.push(`${p?.nombre ?? 'Producto'}: hay ${fmtNumero(hay)} de ${fmtNumero(cant)}`)
    }
    return out
  }
  const opciones = depositos.map((d) => ({ ...d, faltan: faltantes(d.id) }))
  const sugerido = opciones.find((d) => d.faltan.length === 0 && d.sucursal_id === u.sucursal_id)
    ?? opciones.find((d) => d.faltan.length === 0) ?? opciones[0]
  const [elegido, setElegido] = useState<number | null>(null)
  const actual = opciones.find((d) => d.id === (elegido ?? sugerido?.id))

  return (
    <Marco titulo={`Confirmar venta N° ${venta.numero}`} onClose={onClose} pendiente={accion.isPending} error={accion.error}
      desc={muebles
        ? 'Se descuentan el canto y los herrajes del depósito que elijas y quedan los cortes pendientes en Cortes optimizados: las placas se descuentan cuando confirmes que se cortaron. El cliente retira los muebles en la sucursal de ese depósito.'
        : 'Se descuenta el stock del depósito que elijas. El cliente retira en la sucursal de ese depósito.'}
      boton="Confirmar y descontar stock" deshabilitado={!actual || actual.faltan.length > 0 || (muebles && presupuesto.isPending)}
      onSubmit={() => actual && accion.mutate({ accion: 'confirmar', deposito_id: actual.id }, { onSuccess: onClose })}>
      <div className="grid gap-2" role="radiogroup" aria-label="Depósito de salida">
        {opciones.map((d) => {
          const activo = d.id === actual?.id
          return (
            <button key={d.id} type="button" role="radio" aria-checked={activo} onClick={() => setElegido(d.id)}
              className={cn('rounded-lg border p-3 text-left transition-colors hover:bg-accent',
                activo && 'border-primary bg-primary/5 ring-1 ring-primary')}>
              <div className="flex items-center justify-between gap-2">
                <span className="font-medium">{d.nombre}</span>
                <span className="text-sm text-muted-foreground">Sucursal {d.sucursal}</span>
              </div>
              {necesita.size > 0 && (d.faltan.length === 0 ? (
                <p className="mt-1 flex items-center gap-1 text-sm text-emerald-700 dark:text-emerald-400">
                  <CheckCircle2 className="size-4" /> Hay stock de todo
                </p>
              ) : (
                <ul className="mt-1 space-y-0.5 text-sm text-amber-700 dark:text-amber-400">
                  {d.faltan.map((f) => <li key={f} className="flex items-center gap-1"><AlertTriangle className="size-3.5 shrink-0" /> {f}</li>)}
                </ul>
              ))}
            </button>
          )
        })}
      </div>
      {actual && actual.sucursal_id !== u.sucursal_id && (
        <p className="rounded-md bg-sky-500/10 p-3 text-sm text-sky-900 dark:text-sky-200">
          La mercadería sale de <b>{actual.sucursal}</b>: el cliente la retira ahí mostrando la boleta.
        </p>
      )}
    </Marco>
  )
}

export function DialogoPago({ venta, onClose }: { venta: Venta; onClose: () => void }) {
  const accion = useAccionVenta(venta.id)
  const [tipo, setTipo] = useState<Pago['tipo']>(venta.saldo <= 0 ? 'devolucion' : venta.pagado === 0 && venta.estado === 'presupuesto' ? 'sena' : 'pago')
  const [monto, setMonto] = useState(venta.saldo > 0 ? String(venta.saldo).replace('.', ',') : '')
  const [medio, setMedio] = useState(mediosPago[0])
  const [nota, setNota] = useState('')
  const valor = Number(monto.replace(/\./g, '').replace(',', '.'))
  const tope = tipo === 'devolucion' ? venta.pagado : venta.saldo
  return (
    <Marco titulo="Registrar pago" onClose={onClose} pendiente={accion.isPending} error={accion.error}
      desc={<>Total {fmtPesos(venta.total)} · pagado {fmtPesos(venta.pagado)} · saldo <b>{fmtPesos(venta.saldo)}</b></>}
      boton={tipo === 'devolucion' ? 'Registrar devolución' : 'Registrar pago'} deshabilitado={!(valor > 0)}
      onSubmit={() => accion.mutate({ accion: 'pagos', tipo, monto: valor, medio, nota }, { onSuccess: onClose })}>
      <Tabs value={tipo} onValueChange={(v) => setTipo(v as Pago['tipo'])}>
        <TabsList className="w-full">
          <TabsTrigger value="sena">Seña</TabsTrigger>
          <TabsTrigger value="pago">Pago</TabsTrigger>
          {venta.pagado > 0 && <TabsTrigger value="devolucion">Devolución</TabsTrigger>}
        </TabsList>
      </Tabs>
      <div className="grid gap-4 sm:grid-cols-2">
        <label className="grid gap-1.5 text-sm font-medium">
          Monto
          <Input value={monto} onChange={(e) => setMonto(e.target.value)} inputMode="decimal" autoFocus className="text-right" />
          {valor > tope + 0.001 && <span className="text-xs font-normal text-destructive">Supera {fmtPesos(tope)}</span>}
        </label>
        <div className="grid gap-1.5 text-sm font-medium">
          <span>Medio</span>
          <Select value={medio} onValueChange={setMedio}>
            <SelectTrigger className="w-full" aria-label="Medio de pago"><SelectValue /></SelectTrigger>
            <SelectContent>{mediosPago.map((m) => <SelectItem key={m} value={m}>{m}</SelectItem>)}</SelectContent>
          </Select>
        </div>
      </div>
      <label className="grid gap-1.5 text-sm font-medium">
        Nota
        <Textarea value={nota} onChange={(e) => setNota(e.target.value)} rows={2} placeholder="Opcional: n° de operación, quién pagó…" />
      </label>
    </Marco>
  )
}

// DialogoRetiro registra lo que se lleva el cliente ahora. Puede ser una parte.
export function DialogoRetiro({ venta, onClose }: { venta: Venta; onClose: () => void }) {
  const accion = useAccionVenta(venta.id)
  const conPendiente = venta.lineas.filter((l) => pendiente(l) > 0)
  const bloqueado = (tipo: string) => tipo === 'mueble' && venta.estado !== 'listo'
  const [cant, setCant] = useState<Record<string, string>>(() =>
    Object.fromEntries(conPendiente.map((l) => [`${l.tipo}-${l.id}`, bloqueado(l.tipo) ? '0' : String(pendiente(l)).replace('.', ',')])))
  const [nota, setNota] = useState('')
  const items = conPendiente
    .map((l) => ({ linea_id: l.id, tipo: l.tipo, cantidad: Number((cant[`${l.tipo}-${l.id}`] ?? '0').replace(',', '.')) || 0 }))
    .filter((i) => i.cantidad > 0)
  const todo = items.length === conPendiente.length && conPendiente.every((l) => items.find((i) => i.linea_id === l.id && i.tipo === l.tipo)?.cantidad === pendiente(l))

  return (
    <Marco titulo={`Entregar venta N° ${venta.numero}`} onClose={onClose} pendiente={accion.isPending} error={accion.error}
      desc="Cargá lo que se lleva ahora. Si se lleva una parte, el resto queda pendiente en la misma boleta."
      boton={todo ? 'Entregar todo' : 'Registrar entrega parcial'} deshabilitado={items.length === 0}
      onSubmit={() => accion.mutate({ accion: 'retiros', items, nota }, { onSuccess: onClose })}>
      <div className="divide-y rounded-lg border">
        {conPendiente.map((l) => {
          const k = `${l.tipo}-${l.id}`
          return (
            <div key={k} className="flex items-center justify-between gap-3 p-3">
              <div className="min-w-0 text-sm">
                <div className="font-medium">{l.descripcion}</div>
                <div className="text-muted-foreground">
                  Quedan {fmtNumero(pendiente(l))} {unidadCorta[l.unidad] ?? ''}{l.retirado > 0 && ` · ya retiró ${fmtNumero(l.retirado)}`}
                </div>
                {bloqueado(l.tipo) && <div className="text-xs text-amber-700 dark:text-amber-400">Se entrega cuando esté lista</div>}
              </div>
              <Input className="w-24 text-right" inputMode="decimal" value={cant[k]} disabled={bloqueado(l.tipo)}
                aria-label={`Cantidad que retira de ${l.descripcion}`}
                onChange={(e) => setCant({ ...cant, [k]: e.target.value })} />
            </div>
          )
        })}
      </div>
      <label className="grid gap-1.5 text-sm font-medium">
        Nota
        <Textarea value={nota} onChange={(e) => setNota(e.target.value)} rows={2} placeholder="Opcional: quién retiró, patente del flete…" />
      </label>
    </Marco>
  )
}

export function DialogoCancelar({ venta, onClose }: { venta: Venta; onClose: () => void }) {
  const accion = useAccionVenta(venta.id)
  const confirmada = venta.estado !== 'presupuesto'
  return (
    <Marco titulo={`¿Cancelar la venta N° ${venta.numero}?`} onClose={onClose} pendiente={accion.isPending} error={accion.error}
      desc={confirmada ? `La mercadería vuelve al stock de ${venta.deposito}. No se puede deshacer.` : 'El presupuesto queda anulado. No se puede deshacer.'}
      boton="Cancelar venta" destructivo
      onSubmit={() => accion.mutate({ accion: 'cancelar' }, { onSuccess: onClose })} />
  )
}
