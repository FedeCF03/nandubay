import { useState } from 'react'
import { Link, useParams } from 'wouter'
import { ArrowLeft, Ban, CheckCircle2, Loader2, Printer, RefreshCw } from 'lucide-react'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Skeleton } from '@/components/ui/skeleton'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import { usePermiso } from '@/auth/sesion'
import { PlacaSVG } from '@/cortes/PlacaSVG'
import { IconoVeta } from '@/cortes/SelectorVeta'
import { nombreVeta, vetaDe } from '@/cortes/veta'
import { colorEstadoCorte, nombreEstadoCorte, tituloCorte, useAccionCorte, useCorte, type CorteOptimizado } from '@/cortes/api'
import type { Cantos } from '@/cortes/tipos'
import { cn } from '@/lib/utils'
import { fmtNumero } from '@/stock/api'
import { fmtFechaHora } from '@/ventas/api'
import { cuantasPlacas } from '@/presupuestos/api'

const lados = (c: Cantos) => (['l1', 'l2', 'a1', 'a2'] as const).filter((k) => c[k]).map((k) => k.toUpperCase()).join(' ')

export default function CorteDetalle() {
  const { id } = useParams<{ id: string }>()
  const corte = useCorte(Number(id))
  if (corte.isPending) return <Skeleton className="h-96" />
  if (corte.isError) return <p className="text-destructive">{corte.error.message}</p>
  return <Detalle c={corte.data} />
}

type Dialogo = 'confirmar' | 'anular' | null

function Detalle({ c }: { c: CorteOptimizado }) {
  const accion = useAccionCorte(c.id)
  const confirma = usePermiso('cortes.confirmar')
  const genera = usePermiso('cortes.generar')
  const [dialogo, setDialogo] = useState<Dialogo>(null)
  const r = c.resultado
  const pendiente = c.estado === 'pendiente'
  const sobrantes = r.placas.flatMap((pl) => pl.sobrantes ?? [])
  const piezas = c.piezas ?? []
  const retazosUsados = r.placas.filter((pl) => pl.retazo_id)

  const hacer = (a: 'confirmar' | 'recalcular' | 'anular') => accion.mutate(a, { onSuccess: () => setDialogo(null) })

  return (
    <div className="mx-auto max-w-5xl space-y-5">
      <div className="no-imprimir flex flex-wrap items-center justify-between gap-3">
        <div className="flex items-center gap-2">
          <Button variant="ghost" size="icon" asChild aria-label="Volver"><Link href="/cortes-optimizados"><ArrowLeft /></Link></Button>
          <div>
            <div className="flex flex-wrap items-center gap-2">
              <h2 className="text-2xl font-semibold tracking-tight">{tituloCorte(c)}</h2>
              <Badge className={cn('border-0', colorEstadoCorte[c.estado])}>{nombreEstadoCorte[c.estado]}</Badge>
            </div>
            <p className="text-sm text-muted-foreground">
              Corte #{c.id} · {c.placa} · {c.deposito}
              {c.pedido_id && <> · <Link href={`/presupuestos/${c.pedido_id}`} className="underline">ver la venta</Link></>}
            </p>
          </div>
        </div>
        <div className="flex flex-wrap gap-2">
          <Button variant="outline" onClick={() => window.print()}><Printer /> Imprimir</Button>
          {pendiente && genera && (
            <Button variant="outline" onClick={() => hacer('recalcular')} disabled={accion.isPending}
              title="Vuelve a armar el plan con los retazos que hay ahora en el depósito">
              <RefreshCw className={cn(accion.isPending && accion.variables === 'recalcular' && 'animate-spin')} /> Recalcular
            </Button>
          )}
          {pendiente && genera && <Button variant="outline" onClick={() => setDialogo('anular')}><Ban /> Anular</Button>}
          {pendiente && confirma && <Button onClick={() => { accion.reset(); setDialogo('confirmar') }}><CheckCircle2 /> Se cortó</Button>}
        </div>
      </div>
      {accion.isError && !dialogo && <p className="no-imprimir text-sm text-destructive">{accion.error.message}</p>}

      <div className="no-imprimir grid grid-cols-2 gap-3 sm:grid-cols-4">
        <Dato titulo="Placas enteras" valor={String(r.placas_enteras)} />
        <Dato titulo="Retazos" valor={String(r.retazos_usados)} />
        <Dato titulo="Aprovechamiento" valor={`${Math.round(r.aprovechamiento)} %`} />
        <Dato titulo="Canto" valor={`${fmtNumero(c.metros_canto)} m`} />
      </div>

      {c.estado === 'cortado' && c.cortado_en && (
        <p className="no-imprimir text-sm text-muted-foreground">
          Cortado el {fmtFechaHora(c.cortado_en)}{c.cortado_por && ` por ${c.cortado_por}`}. Se descontaron {cuantasPlacas(r.placas_enteras, 0)} y
          {sobrantes.length ? ` quedaron ${sobrantes.length} retazos nuevos en Stock.` : ' no quedaron retazos.'}
        </p>
      )}

      <Card className="placa-hoja gap-3">
        <CardHeader><CardTitle>Piezas</CardTitle></CardHeader>
        <CardContent>
          <div className="-mx-6 overflow-x-auto">
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead className="pl-6">Pieza</TableHead>
                  <TableHead className="text-right">Largo × ancho</TableHead>
                  <TableHead className="text-right">Cant.</TableHead>
                  <TableHead>Veta</TableHead>
                  <TableHead className="pr-6">Canto</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {piezas.map((p) => (
                  <TableRow key={p.id}>
                    <TableCell className="pl-6 font-medium">{p.etiqueta}</TableCell>
                    <TableCell className="text-right tabular-nums">{p.largo} × {p.ancho}</TableCell>
                    <TableCell className="text-right tabular-nums">{p.cantidad}</TableCell>
                    <TableCell className="text-sm whitespace-nowrap">
                      {c.parametros.tiene_veta && vetaDe(p) !== 'no'
                        ? <span className="inline-flex items-center gap-1"><IconoVeta veta={vetaDe(p)} /> {nombreVeta[vetaDe(p)]}</span>
                        : <span className="text-muted-foreground">—</span>}
                    </TableCell>
                    <TableCell className="pr-6 font-mono text-sm">{lados(p.cantos) || '—'}</TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </div>
          <p className="mt-3 text-sm">
            <b>Usar:</b> {cuantasPlacas(r.placas_enteras, r.retazos_usados)} de {c.placa}
            {retazosUsados.length > 0 && <span className="text-muted-foreground"> (retazos {retazosUsados.map((pl) => `#${pl.retazo_id}`).join(', ')})</span>}
          </p>
        </CardContent>
      </Card>

      {r.placas.map((pl) => (
        <Card key={pl.indice} className="placa-hoja gap-3">
          <CardHeader className="flex flex-row flex-wrap items-center justify-between gap-2">
            <CardTitle className="flex flex-wrap items-center gap-2">
              {pl.retazo_id ? `Retazo #${pl.retazo_id}` : `Placa entera ${pl.indice}`}
              <span className="text-sm font-normal text-muted-foreground">{pl.largo} × {pl.ancho} mm · {pl.indice} de {r.placas.length}</span>
            </CardTitle>
            <div className="flex gap-2">
              <Badge variant="outline">{pl.piezas.length} piezas</Badge>
              <Badge>{Math.round(pl.aprovechamiento)} % usado</Badge>
            </div>
          </CardHeader>
          <CardContent className="space-y-2">
            <div className="-mx-2 overflow-x-auto px-2 print:overflow-visible">
              <div className="min-w-[640px] print:min-w-0">
                <PlacaSVG placa={pl} piezas={piezas} tieneVeta={c.parametros.tiene_veta} />
              </div>
            </div>
            {(pl.sobrantes ?? []).length > 0 && (
              <p className="text-sm"><b>Guardar como retazo:</b> {(pl.sobrantes ?? []).map((s) => `${s.largo} × ${s.ancho}`).join(' · ')}</p>
            )}
          </CardContent>
        </Card>
      ))}

      <Dialog open={dialogo !== null} onOpenChange={(o) => !o && setDialogo(null)}>
        <DialogContent>
          {dialogo === 'confirmar' ? (
            <>
              <DialogHeader>
                <DialogTitle>¿Ya se cortó?</DialogTitle>
                <DialogDescription>Esto cambia el stock de {c.deposito} y no se puede deshacer desde acá.</DialogDescription>
              </DialogHeader>
              <ul className="list-disc space-y-1 pl-5 text-sm">
                <li>Se descuentan <b>{cuantasPlacas(r.placas_enteras, 0)}</b> de {c.placa}.</li>
                {retazosUsados.length > 0 && <li>Se marcan como usados los retazos {retazosUsados.map((pl) => `#${pl.retazo_id}`).join(', ')}.</li>}
                <li>{sobrantes.length ? <>Se guardan <b>{sobrantes.length}</b> sobrantes como retazos: {sobrantes.map((s) => `${s.largo} × ${s.ancho}`).join(', ')}.</> : 'No quedan sobrantes para guardar.'}</li>
              </ul>
            </>
          ) : (
            <DialogHeader>
              <DialogTitle>¿Anular el corte?</DialogTitle>
              <DialogDescription>No se toca el stock. El corte queda en la lista de anulados.</DialogDescription>
            </DialogHeader>
          )}
          {accion.isError && <p className="text-sm text-destructive">{accion.error.message}</p>}
          <DialogFooter>
            <Button variant="outline" onClick={() => setDialogo(null)}>Volver</Button>
            <Button variant={dialogo === 'anular' ? 'destructive' : 'default'} disabled={accion.isPending}
              onClick={() => hacer(dialogo === 'anular' ? 'anular' : 'confirmar')}>
              {accion.isPending && <Loader2 className="animate-spin" />}
              {dialogo === 'anular' ? 'Anular' : 'Sí, se cortó'}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  )
}

function Dato({ titulo, valor }: { titulo: string; valor: string }) {
  return (
    <div className="rounded-xl border bg-card px-4 py-3">
      <div className="text-xs text-muted-foreground">{titulo}</div>
      <div className="text-lg font-semibold tabular-nums">{valor}</div>
    </div>
  )
}
