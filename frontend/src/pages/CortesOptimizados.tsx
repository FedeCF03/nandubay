import { Link, useSearch } from 'wouter'
import { ChevronRight, Layers, Recycle } from 'lucide-react'
import { Badge } from '@/components/ui/badge'
import { Card, CardContent } from '@/components/ui/card'
import { Skeleton } from '@/components/ui/skeleton'
import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { useGuardado } from '@/hooks/use-guardado'
import { cn } from '@/lib/utils'
import { fmtFechaHora } from '@/ventas/api'
import { cuantasPlacas } from '@/presupuestos/api'
import { colorEstadoCorte, nombreEstadoCorte, tituloCorte, useCortes, type CorteOptimizado } from '@/cortes/api'

// CortesOptimizados lista los planes de corte guardados. Los pendientes van
// primero: son los que el taller tiene que cortar.
export default function CortesOptimizados() {
  const [estado, setEstado] = useGuardado('nandubay.cortes-optimizados.estado', 'pendiente')
  // ?pedido=ID: solo los cortes de una venta.
  const pedido = Number(new URLSearchParams(useSearch()).get('pedido')) || 0
  const cortes = useCortes(pedido ? '' : estado)
  const lista = (cortes.data ?? []).filter((c) => !pedido || c.pedido_id === pedido)

  return (
    <div className="mx-auto max-w-5xl space-y-5">
      <div>
        <h2 className="text-2xl font-semibold tracking-tight">Cortes optimizados</h2>
        <p className="text-muted-foreground">
          Planes guardados para cortar. Al confirmar que se cortó se descuentan las placas y los sobrantes quedan como retazos.
        </p>
      </div>
      {pedido ? (
        <p className="text-sm">Cortes de una venta · <Link href="/cortes-optimizados" className="underline">ver todos</Link></p>
      ) : (
        <div className="-mx-4 overflow-x-auto px-4">
          <Tabs value={estado || 'todos'} onValueChange={(v) => setEstado(v === 'todos' ? '' : v)}>
            <TabsList>
              <TabsTrigger value="pendiente">Pendientes</TabsTrigger>
              <TabsTrigger value="cortado">Cortados</TabsTrigger>
              <TabsTrigger value="anulado">Anulados</TabsTrigger>
              <TabsTrigger value="todos">Todos</TabsTrigger>
            </TabsList>
          </Tabs>
        </div>
      )}
      {cortes.isPending ? <Skeleton className="h-40" />
        : cortes.isError ? <p className="text-destructive">{cortes.error.message}</p>
          : lista.length === 0 ? (
            <Card><CardContent className="py-10 text-center text-sm text-muted-foreground">
              {estado === 'pendiente' && !pedido
                ? <>No hay cortes pendientes. Se crean al confirmar un presupuesto de muebles o desde el <Link href="/cortes" className="underline">optimizador</Link> con "Guardar para cortar".</>
                : 'No hay cortes.'}
            </CardContent></Card>
          ) : (
            <ul className="space-y-3">{lista.map((c) => <li key={c.id}><Fila c={c} /></li>)}</ul>
          )}
    </div>
  )
}

function Fila({ c }: { c: CorteOptimizado }) {
  const r = c.resultado
  return (
    <Link href={`/cortes-optimizados/${c.id}`}
      className="flex items-center gap-4 rounded-xl border bg-card p-4 transition-colors hover:bg-accent/50">
      <div className="min-w-0 flex-1 space-y-1">
        <div className="flex flex-wrap items-center gap-2">
          <span className="font-mono text-sm text-muted-foreground">#{c.id}</span>
          <span className="truncate font-medium">{tituloCorte(c)}</span>
          <Badge className={cn('border-0', colorEstadoCorte[c.estado])}>{nombreEstadoCorte[c.estado]}</Badge>
        </div>
        <div className="flex flex-wrap gap-x-4 gap-y-1 text-sm text-muted-foreground">
          <span className="inline-flex items-center gap-1"><Layers className="size-3.5" /> {c.placa}</span>
          <span className="inline-flex items-center gap-1"><Recycle className="size-3.5" /> {cuantasPlacas(r.placas_enteras, r.retazos_usados)}</span>
          <span>{c.deposito}</span>
          <span>{Math.round(r.aprovechamiento)} % aprovechado</span>
        </div>
        <div className="text-xs text-muted-foreground">
          {c.estado === 'cortado' && c.cortado_en
            ? `Cortado ${fmtFechaHora(c.cortado_en)}${c.cortado_por ? ` por ${c.cortado_por}` : ''}`
            : `Creado ${fmtFechaHora(c.creado_en)}${c.creado_por ? ` por ${c.creado_por}` : ''}`}
        </div>
      </div>
      <ChevronRight className="size-5 shrink-0 text-muted-foreground" />
    </Link>
  )
}
