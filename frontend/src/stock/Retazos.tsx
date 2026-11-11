import { useState, type FormEvent } from 'react'
import { Loader2, Plus, Recycle, Trash2 } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Card } from '@/components/ui/card'
import {
  Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle,
} from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Skeleton } from '@/components/ui/skeleton'
import { usePermiso, useUsuario } from '@/auth/sesion'
import { fmtFecha } from '@/ventas/api'
import { useDepositos, useProductos, useRetazoAcciones, useRetazos } from './api'

// Retazos disponibles: los sobrantes que deja cada venta confirmada y los que
// se cargan a mano. El optimizador los usa antes que una placa entera.
export function Retazos() {
  const [placa, setPlaca] = useState(0)
  const [deposito, setDeposito] = useState(0)
  const [nuevo, setNuevo] = useState(false)
  const retazos = useRetazos(placa, deposito)
  const placas = useProductos({ tipo: 'placa', buscar: '' }).data ?? []
  const depositos = useDepositos().data ?? []
  const { descartar } = useRetazoAcciones()
  const edita = usePermiso('stock.editar')
  const lista = retazos.data ?? []

  return (
    <div className="space-y-4">
      <div className="flex flex-col gap-2 sm:flex-row sm:items-center">
        <Select value={String(placa)} onValueChange={(v) => setPlaca(Number(v))}>
          <SelectTrigger className="w-full sm:w-64" aria-label="Placa"><SelectValue /></SelectTrigger>
          <SelectContent>
            <SelectItem value="0">Todas las placas</SelectItem>
            {placas.map((p) => <SelectItem key={p.id} value={String(p.id)}>{p.nombre}</SelectItem>)}
          </SelectContent>
        </Select>
        {depositos.length > 1 && (
          <Select value={String(deposito)} onValueChange={(v) => setDeposito(Number(v))}>
            <SelectTrigger className="w-full sm:w-56" aria-label="Depósito"><SelectValue /></SelectTrigger>
            <SelectContent>
              <SelectItem value="0">Todos los depósitos</SelectItem>
              {depositos.map((d) => <SelectItem key={d.id} value={String(d.id)}>{d.nombre}</SelectItem>)}
            </SelectContent>
          </Select>
        )}
        <div className="flex-1" />
        {edita && <Button onClick={() => setNuevo(true)}><Plus /> Agregar retazo</Button>}
      </div>
      <p className="text-sm text-muted-foreground">
        El optimizador usa estos retazos antes que una placa entera. Al confirmar una venta de muebles se marcan los que se usan y los sobrantes quedan como retazos nuevos. Escribí el número en el retazo para encontrarlo.
      </p>
      {descartar.error && <p className="text-sm text-destructive">{descartar.error.message}</p>}

      {retazos.isPending ? (
        <div className="space-y-2">{Array.from({ length: 4 }, (_, i) => <Skeleton key={i} className="h-14" />)}</div>
      ) : lista.length === 0 ? (
        <div className="flex flex-col items-center gap-2 rounded-xl border border-dashed py-16 text-center">
          <Recycle className="size-10 text-muted-foreground" />
          <p className="font-medium">No hay retazos disponibles{placa || deposito ? ' con ese filtro' : ''}.</p>
        </div>
      ) : (
        <div className="grid gap-2 sm:grid-cols-2 xl:grid-cols-3">
          {lista.map((r) => (
            <Card key={r.id} className="min-w-0 flex-row items-center gap-3 px-4 py-3">
              <div className="flex size-12 shrink-0 items-center justify-center rounded-md bg-muted font-mono text-sm font-semibold">#{r.id}</div>
              <div className="min-w-0 flex-1">
                <div className="font-medium tabular-nums">{r.largo} × {r.ancho} mm</div>
                <div className="truncate text-sm">{r.placa}</div>
                <div className="truncate text-xs text-muted-foreground">
                  {r.deposito} · {r.origen ? `sobrante de ${r.origen}` : 'cargado a mano'} · {fmtFecha(r.creado_en)}
                  {r.nota && ` · ${r.nota}`}
                </div>
              </div>
              {edita && (
                <Button variant="ghost" size="icon" aria-label={`Descartar retazo ${r.id}`} title="Descartar (se tiró o se usó)"
                  className="text-muted-foreground" disabled={descartar.isPending}
                  onClick={() => descartar.mutate(r.id)}><Trash2 /></Button>
              )}
            </Card>
          ))}
        </div>
      )}
      {nuevo && <NuevoRetazo placaInicial={placa} onClose={() => setNuevo(false)} />}
    </div>
  )
}

function NuevoRetazo({ placaInicial, onClose }: { placaInicial: number; onClose: () => void }) {
  const u = useUsuario()
  const placas = useProductos({ tipo: 'placa', buscar: '' }).data ?? []
  const depositos = useDepositos().data ?? []
  const propio = depositos.find((d) => d.sucursal_id === u.sucursal_id) ?? depositos[0]
  const [placa, setPlaca] = useState(placaInicial ? String(placaInicial) : '')
  const [deposito, setDeposito] = useState('')
  const [largo, setLargo] = useState('')
  const [ancho, setAncho] = useState('')
  const [nota, setNota] = useState('')
  const { crear } = useRetazoAcciones()
  const dep = deposito || (propio ? String(propio.id) : '')
  const pl = placas.find((p) => String(p.id) === placa)

  const enviar = (e: FormEvent) => {
    e.preventDefault()
    crear.mutate({ placa_id: Number(placa), deposito_id: Number(dep), largo: Number(largo), ancho: Number(ancho), nota },
      { onSuccess: onClose })
  }
  return (
    <Dialog open onOpenChange={(o) => !o && onClose()}>
      <DialogContent>
        <form onSubmit={enviar} className="grid gap-4">
          <DialogHeader>
            <DialogTitle>Agregar retazo</DialogTitle>
            <DialogDescription>Un pedazo de placa que sirve para cortar piezas.</DialogDescription>
          </DialogHeader>
          <div className="grid gap-1.5">
            <Label>Placa</Label>
            <Select value={placa} onValueChange={setPlaca}>
              <SelectTrigger className="w-full" aria-label="Placa del retazo"><SelectValue placeholder="Elegí la placa" /></SelectTrigger>
              <SelectContent>{placas.map((p) => <SelectItem key={p.id} value={String(p.id)}>{p.nombre}</SelectItem>)}</SelectContent>
            </Select>
          </div>
          {depositos.length > 1 && (
            <div className="grid gap-1.5">
              <Label>Depósito</Label>
              <Select value={dep} onValueChange={setDeposito}>
                <SelectTrigger className="w-full" aria-label="Depósito del retazo"><SelectValue /></SelectTrigger>
                <SelectContent>{depositos.map((d) => <SelectItem key={d.id} value={String(d.id)}>{d.nombre}</SelectItem>)}</SelectContent>
              </Select>
            </div>
          )}
          <div className="grid grid-cols-2 gap-3">
            <div className="grid gap-1.5">
              <Label htmlFor="r-largo">Largo (mm){pl?.tiene_veta && ', con la veta'}</Label>
              <Input id="r-largo" inputMode="numeric" value={largo} onChange={(e) => setLargo(e.target.value.replace(/\D/g, ''))} />
            </div>
            <div className="grid gap-1.5">
              <Label htmlFor="r-ancho">Ancho (mm)</Label>
              <Input id="r-ancho" inputMode="numeric" value={ancho} onChange={(e) => setAncho(e.target.value.replace(/\D/g, ''))} />
            </div>
          </div>
          <div className="grid gap-1.5">
            <Label htmlFor="r-nota">Nota (opcional)</Label>
            <Input id="r-nota" value={nota} placeholder="Ej: estante de arriba" onChange={(e) => setNota(e.target.value)} />
          </div>
          {crear.error && <p className="text-sm text-destructive">{crear.error.message}</p>}
          <DialogFooter>
            <Button type="button" variant="outline" onClick={onClose}>Cancelar</Button>
            <Button type="submit" disabled={!placa || !dep || !largo || !ancho || crear.isPending}>
              {crear.isPending && <Loader2 className="animate-spin" />} Guardar retazo
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}
