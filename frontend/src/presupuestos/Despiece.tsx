import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Skeleton } from '@/components/ui/skeleton'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import { fmtNumero, unidadCorta, useProductos } from '@/stock/api'
import { medidas } from '@/ventas/api'
import { tieneCanto, usePresupuesto } from './api'
import { PlanesDeCorte, ResumenMateriales } from './Materiales'

const lados = (c: { l1: boolean; l2: boolean; a1: boolean; a2: boolean }) =>
  (['l1', 'l2', 'a1', 'a2'] as const).filter((k) => c[k]).map((k) => k.toUpperCase()).join(' ')

// DespieceVenta muestra, dentro de un presupuesto de muebles, los cortes de
// cada mueble, lo que hay que usar del stock y cómo se corta cada placa.
export function DespieceVenta({ id }: { id: number }) {
  const p = usePresupuesto(id)
  const productos = useProductos({ tipo: '', buscar: '' })
  if (p.isPending) return <Skeleton className="h-64" />
  if (p.isError) return <p className="text-destructive">{p.error.message}</p>
  const { muebles, materiales, error_materiales } = p.data
  const nombre = new Map((productos.data ?? []).map((x) => [x.id, x]))
  if (muebles.every((m) => m.piezas.length === 0 && m.insumos.length === 0)) return null

  return (
    <div className="space-y-5">
      <div className="grid gap-5 lg:grid-cols-[minmax(0,1fr)_20rem]">
        <div className="min-w-0 space-y-5">
          {muebles.map((m, i) => (
            <Card key={i} className="min-w-0 gap-3">
              <CardHeader>
                <CardTitle>{m.cantidad > 1 && `${m.cantidad} × `}{m.nombre}</CardTitle>
                <p className="text-sm text-muted-foreground">{medidas(m) || 'Sin medidas generales'}</p>
              </CardHeader>
              <CardContent className="space-y-4">
                {m.piezas.length > 0 && (
                  <div className="-mx-6 overflow-x-auto">
                    <Table>
                      <TableHeader>
                        <TableRow>
                          <TableHead className="pl-6">Pieza</TableHead>
                          <TableHead>Placa</TableHead>
                          <TableHead className="text-right">Medida</TableHead>
                          <TableHead className="text-right">Cant.</TableHead>
                          <TableHead className="pr-6">Canto</TableHead>
                        </TableRow>
                      </TableHeader>
                      <TableBody>
                        {m.piezas.map((pz, j) => (
                          <TableRow key={j}>
                            <TableCell className="pl-6 font-medium">{pz.nombre || `Pieza ${j + 1}`}</TableCell>
                            <TableCell className="max-w-40 truncate">{nombre.get(pz.placa_id)?.nombre ?? '—'}</TableCell>
                            <TableCell className="text-right whitespace-nowrap tabular-nums">{pz.largo} × {pz.ancho}</TableCell>
                            <TableCell className="text-right tabular-nums">{pz.cantidad}</TableCell>
                            <TableCell className="pr-6 text-sm">
                              {tieneCanto(pz.cantos)
                                ? <><span className="font-mono">{lados(pz.cantos)}</span> <span className="text-muted-foreground">{nombre.get(pz.canto_id ?? 0)?.nombre}</span></>
                                : <span className="text-muted-foreground">—</span>}
                            </TableCell>
                          </TableRow>
                        ))}
                      </TableBody>
                    </Table>
                  </div>
                )}
                {m.insumos.length > 0 && (
                  <div>
                    <div className="mb-1 text-sm font-medium">Herrajes e insumos (por mueble)</div>
                    <ul className="text-sm text-muted-foreground">
                      {m.insumos.map((x) => {
                        const pr = nombre.get(x.producto_id)
                        return <li key={x.producto_id}>{fmtNumero(x.cantidad)} {pr ? unidadCorta[pr.unidad] : ''} {pr?.nombre ?? `Producto ${x.producto_id}`}</li>
                      })}
                    </ul>
                  </div>
                )}
              </CardContent>
            </Card>
          ))}
        </div>
        <Card>
          <CardHeader><CardTitle>Materiales a usar</CardTitle></CardHeader>
          <CardContent>
            {materiales ? <ResumenMateriales mat={materiales} />
              : <p className="text-sm text-destructive">{error_materiales || 'No se pudieron calcular.'}</p>}
          </CardContent>
        </Card>
      </div>
      {materiales && materiales.placas.length > 0 && (
        <Card>
          <CardHeader><CardTitle>Plano de cortes</CardTitle></CardHeader>
          <CardContent><PlanesDeCorte mat={materiales} muebles={muebles} /></CardContent>
        </Card>
      )}
    </div>
  )
}
