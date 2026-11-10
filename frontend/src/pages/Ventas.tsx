import { useState } from 'react'
import { Link, useLocation } from 'wouter'
import { BarChart3, Plus, Receipt, Search } from 'lucide-react'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Skeleton } from '@/components/ui/skeleton'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { usePermiso } from '@/auth/sesion'
import { cn } from '@/lib/utils'
import { fmtPesos } from '@/stock/api'
import { colorEstado, fmtFecha, nombreEstado, numeroVenta, useVentas, type ResumenVenta, type TipoVenta } from '@/ventas/api'
import { ProductosVendidos } from '@/ventas/ProductosVendidos'

export function EstadoBadge({ estado, className }: { estado: ResumenVenta['estado']; className?: string }) {
  return <Badge variant="secondary" className={cn('border-0', colorEstado[estado], className)}>{nombreEstado[estado]}</Badge>
}

function Saldo({ v }: { v: ResumenVenta }) {
  if (v.estado === 'presupuesto' || v.estado === 'cancelado') return <span className="text-muted-foreground">—</span>
  if (v.saldo <= 0) return <span className="font-medium text-emerald-700 dark:text-emerald-400">Pagada</span>
  return <span className="font-medium text-amber-700 tabular-nums dark:text-amber-400">{fmtPesos(v.saldo)}</span>
}

export default function Ventas() {
  const [vista, setVista] = useState<'boletas' | 'vendidos'>('boletas')
  const crea = usePermiso('pedidos.editar')
  return (
    <div className="space-y-5">
      <div className="flex flex-wrap items-end justify-between gap-3">
        <div>
          <h2 className="text-2xl font-semibold tracking-tight">Ventas de mostrador</h2>
          <p className="text-muted-foreground">Placas, cantos y herrajes del stock: boletas, cobros y retiros.</p>
        </div>
        {crea && (
          <Button asChild><Link href="/ventas/nueva"><Plus /> Nueva venta</Link></Button>
        )}
      </div>
      <Tabs value={vista} onValueChange={(v) => setVista(v as typeof vista)}>
        <TabsList>
          <TabsTrigger value="boletas"><Receipt /> Historial de boletas</TabsTrigger>
          <TabsTrigger value="vendidos"><BarChart3 /> Productos vendidos</TabsTrigger>
        </TabsList>
      </Tabs>
      {vista === 'boletas' ? <ListaVentas tipo="mostrador" /> : <ProductosVendidos />}
    </div>
  )
}

// ListaVentas es el historial de ventas de un tipo, con filtros por estado.
export function ListaVentas({ tipo }: { tipo: TipoVenta }) {
  const base = tipo === 'muebles' ? '/presupuestos' : '/ventas'
  const [estado, setEstado] = useState('')
  const [buscar, setBuscar] = useState('')
  const [, navegar] = useLocation()
  const ventas = useVentas(tipo, estado, buscar)
  const crea = usePermiso('pedidos.editar')
  const lista = ventas.data ?? []

  return (
    <div className="space-y-4">

      <div className="flex flex-col gap-3 md:flex-row md:items-center">
        <div className="-mx-4 overflow-x-auto px-4 md:mx-0 md:px-0">
          <Tabs value={estado || 'todas'} onValueChange={(v) => setEstado(v === 'todas' ? '' : v)}>
            <TabsList>
              <TabsTrigger value="todas">Todas</TabsTrigger>
              <TabsTrigger value="presupuesto">Presupuestos</TabsTrigger>
              <TabsTrigger value="a_retirar">En curso</TabsTrigger>
              <TabsTrigger value="entregado">Entregadas</TabsTrigger>
            </TabsList>
          </Tabs>
        </div>
        <div className="relative flex-1">
          <Search className="pointer-events-none absolute top-1/2 left-2.5 size-4 -translate-y-1/2 text-muted-foreground" />
          <Input className="pl-8" placeholder="N° de boleta o cliente" value={buscar} inputMode="search"
            onChange={(e) => setBuscar(e.target.value)} />
        </div>
      </div>

      {ventas.isPending ? (
        <div className="space-y-2">{Array.from({ length: 5 }, (_, i) => <Skeleton key={i} className="h-14" />)}</div>
      ) : lista.length === 0 ? (
        <div className="flex flex-col items-center gap-2 rounded-xl border border-dashed py-16 text-center">
          <Receipt className="size-10 text-muted-foreground" />
          <p className="font-medium">{buscar || estado ? 'No hay ventas que coincidan.' : tipo === 'muebles' ? 'Todavía no hay presupuestos de muebles.' : 'Todavía no hay ventas.'}</p>
          {crea && !buscar && !estado && (
            <Button variant="outline" asChild>
              <Link href={tipo === 'muebles' ? '/presupuestos/nuevo' : '/ventas/nueva'}><Plus /> {tipo === 'muebles' ? 'Hacer el primer presupuesto' : 'Hacer la primera venta'}</Link>
            </Button>
          )}
        </div>
      ) : (
        <>
          <Card className="hidden py-0 md:block">
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead className="pl-4">N°</TableHead>
                  <TableHead>Cliente</TableHead>
                  <TableHead>Estado</TableHead>
                  <TableHead>Vendida en</TableHead>
                  <TableHead>Retira en</TableHead>
                  <TableHead className="text-right">Total</TableHead>
                  <TableHead className="pr-4 text-right">Saldo</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {lista.map((v) => (
                  <TableRow key={v.id} className="cursor-pointer" onClick={() => navegar(`${base}/${v.id}`)}>
                    <TableCell className="pl-4 font-mono text-sm">
                      <Link href={`${base}/${v.id}`} className="hover:underline">{numeroVenta(v.numero)}</Link>
                      <div className="font-sans text-xs text-muted-foreground">{fmtFecha(v.fecha)}</div>
                    </TableCell>
                    <TableCell className="font-medium">{v.cliente}</TableCell>
                    <TableCell><EstadoBadge estado={v.estado} /></TableCell>
                    <TableCell>{v.sucursal}</TableCell>
                    <TableCell>{v.retira || <span className="text-muted-foreground">—</span>}</TableCell>
                    <TableCell className="text-right tabular-nums">{fmtPesos(v.total)}</TableCell>
                    <TableCell className="pr-4 text-right"><Saldo v={v} /></TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </Card>

          <div className="space-y-2 md:hidden">
            {lista.map((v) => (
              <Link key={v.id} href={`${base}/${v.id}`}>
                <Card className="gap-1 px-4 py-3 active:bg-accent">
                  <div className="flex items-center justify-between gap-2">
                    <span className="truncate font-medium">{v.cliente}</span>
                    <EstadoBadge estado={v.estado} />
                  </div>
                  <div className="flex items-center justify-between text-sm text-muted-foreground">
                    <span className="font-mono">N° {numeroVenta(v.numero)} · {fmtFecha(v.fecha)}</span>
                    <span className="text-foreground tabular-nums">{fmtPesos(v.total)}</span>
                  </div>
                  {v.retira && (
                    <div className="flex items-center justify-between text-sm">
                      <span className="text-muted-foreground">Retira en {v.retira}</span>
                      <Saldo v={v} />
                    </div>
                  )}
                </Card>
              </Link>
            ))}
          </div>
        </>
      )}
    </div>
  )
}
