import { useMutation } from '@tanstack/react-query'
import { AlertTriangle, Layers, Loader2, Plus, Printer, Recycle, Ruler, Scissors, Trash2 } from 'lucide-react'
import { postJSON } from '@/api/cliente'
import { usePermiso } from '@/auth/sesion'
import { useGuardado } from '@/hooks/use-guardado'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Checkbox } from '@/components/ui/checkbox'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { PlacaSVG } from './PlacaSVG'
import { SelectorCantos } from './SelectorCantos'
import type { Parametros, Pieza, Resultado } from './tipos'

let siguienteId = 4
const nuevaPieza = (): Pieza => ({
  id: String(siguienteId++), etiqueta: '', largo: 0, ancho: 0, cantidad: 1, respeta_veta: true,
  cantos: { l1: false, l2: false, a1: false, a2: false },
})

const ejemplo: Pieza[] = [
  { id: '1', etiqueta: 'Lateral', largo: 720, ancho: 560, cantidad: 2, respeta_veta: true,
    cantos: { l1: true, l2: false, a1: false, a2: false } },
  { id: '2', etiqueta: 'Puerta', largo: 716, ancho: 396, cantidad: 2, respeta_veta: true,
    cantos: { l1: true, l2: true, a1: true, a2: true } },
  { id: '3', etiqueta: 'Piso', largo: 764, ancho: 560, cantidad: 1, respeta_veta: true,
    cantos: { l1: true, l2: false, a1: false, a2: false } },
]

const num = (v: string) => Math.max(0, Number(v) || 0)

function Campo({ id, label, valor, onChange, sufijo }: {
  id: string; label: string; valor: number; onChange: (n: number) => void; sufijo?: string
}) {
  return (
    <div className="grid gap-1.5">
      <Label htmlFor={id} className="text-xs text-muted-foreground">{label}</Label>
      <div className="relative">
        <Input id={id} type="number" inputMode="numeric" value={valor}
          onChange={(e) => onChange(num(e.target.value))} className={sufijo ? 'pr-9' : ''} />
        {sufijo && <span className="pointer-events-none absolute inset-y-0 right-3 flex items-center text-xs text-muted-foreground">{sufijo}</span>}
      </div>
    </div>
  )
}

function Dato({ icono: Icono, titulo, valor }: { icono: typeof Layers; titulo: string; valor: string }) {
  return (
    <Card className="gap-1 py-4">
      <CardContent className="flex items-center gap-3 px-4">
        <div className="flex size-9 items-center justify-center rounded-lg bg-accent text-accent-foreground">
          <Icono className="size-4" />
        </div>
        <div>
          <div className="text-xs text-muted-foreground">{titulo}</div>
          <div className="text-lg font-semibold tabular-nums">{valor}</div>
        </div>
      </CardContent>
    </Card>
  )
}

export function Optimizador() {
  const [params, setParams] = useGuardado<Parametros>('nandubay.cortes.placa', {
    placa_largo: 2600, placa_ancho: 1830, tiene_veta: true, kerf: 3, refilado: 0,
    sobrante_min_largo: 300, sobrante_min_ancho: 150,
  })
  const [piezas, setPiezas] = useGuardado<Pieza[]>('nandubay.cortes.piezas', ejemplo)
  // Las piezas nuevas tienen que tener un id distinto de las guardadas.
  siguienteId = Math.max(siguienteId, ...piezas.map((p) => Number(p.id) + 1 || 0))
  const puedeGenerar = usePermiso('cortes.generar')

  const optimizar = useMutation({
    mutationFn: () =>
      postJSON<Resultado>('/api/v1/cortes/optimizar', {
        parametros: params,
        // Sin nombre, la pieza se llama como su renglón (lo mismo que muestra el campo vacío).
        piezas: piezas
          .map((p, i) => ({ ...p, etiqueta: p.etiqueta.trim() || `Pieza ${i + 1}` }))
          .filter((p) => p.largo > 0 && p.ancho > 0),
      }),
  })
  const res = optimizar.data
  const setPieza = (i: number, cambios: Partial<Pieza>) =>
    setPiezas((ps) => ps.map((p, j) => (j === i ? { ...p, ...cambios } : p)))
  const totalPiezas = piezas.reduce((t, p) => t + (p.largo && p.ancho ? p.cantidad : 0), 0)

  return (
    <div className="space-y-6">
      <div className="no-imprimir">
        <h2 className="text-2xl font-semibold tracking-tight">Optimizador de cortes</h2>
        <p className="text-muted-foreground">Cargá las piezas y te armo el plano para la escuadradora.</p>
      </div>

      <Card className="no-imprimir">
        <CardHeader>
          <CardTitle>Placa</CardTitle>
          <CardDescription>Medidas de la placa entera. El largo va en el sentido de la veta.</CardDescription>
        </CardHeader>
        <CardContent className="grid grid-cols-2 gap-4 sm:grid-cols-4 lg:grid-cols-5">
          <Campo id="largo" label="Largo" sufijo="mm" valor={params.placa_largo}
            onChange={(n) => setParams({ ...params, placa_largo: n })} />
          <Campo id="ancho" label="Ancho" sufijo="mm" valor={params.placa_ancho}
            onChange={(n) => setParams({ ...params, placa_ancho: n })} />
          <Campo id="kerf" label="Espesor de sierra" sufijo="mm" valor={params.kerf}
            onChange={(n) => setParams({ ...params, kerf: n })} />
          <Campo id="refilado" label="Refilado por borde" sufijo="mm" valor={params.refilado}
            onChange={(n) => setParams({ ...params, refilado: n })} />
          <div className="col-span-2 flex items-center gap-2 sm:col-span-4 lg:col-span-1 lg:self-end lg:pb-2">
            <Checkbox id="veta" checked={params.tiene_veta}
              onCheckedChange={(v) => setParams({ ...params, tiene_veta: v === true })} />
            <Label htmlFor="veta">La placa tiene veta</Label>
          </div>
        </CardContent>
      </Card>

      <Card className="no-imprimir">
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            Piezas <Badge variant="secondary">{totalPiezas}</Badge>
          </CardTitle>
          <CardDescription>
            Tocá los lados del dibujito para marcar dónde lleva canto.
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-3">
          <div className="hidden grid-cols-[1fr_6rem_6rem_5rem_4rem_4rem_2.5rem] gap-3 px-1 text-xs text-muted-foreground md:grid">
            <span>Nombre (opcional)</span><span>Largo</span><span>Ancho</span><span>Cant.</span>
            <span>Veta</span><span>Cantos</span><span />
          </div>
          {piezas.map((p, i) => (
            <div key={p.id}
              className="grid grid-cols-3 items-end gap-3 rounded-lg border p-3 md:grid-cols-[1fr_6rem_6rem_5rem_4rem_4rem_2.5rem] md:items-center md:border-0 md:p-1">
              <div className="col-span-3 md:col-span-1">
                <Label className="mb-1.5 text-xs text-muted-foreground md:hidden">Nombre (opcional)</Label>
                <Input value={p.etiqueta} placeholder={`Pieza ${i + 1}`} aria-label="Nombre (opcional)"
                  onChange={(e) => setPieza(i, { etiqueta: e.target.value })} />
              </div>
              {(['largo', 'ancho', 'cantidad'] as const).map((k) => (
                <div key={k}>
                  <Label className="mb-1.5 text-xs text-muted-foreground capitalize md:hidden">{k}</Label>
                  <Input type="number" inputMode="numeric" value={p[k] || ''} aria-label={k}
                    onChange={(e) => setPieza(i, { [k]: num(e.target.value) })} />
                </div>
              ))}
              <div className="col-span-3 flex items-center justify-between gap-4 md:contents">
                <label className="flex items-center gap-2 text-sm md:justify-center">
                  <Checkbox checked={p.respeta_veta} disabled={!params.tiene_veta} aria-label="Respeta veta"
                    onCheckedChange={(v) => setPieza(i, { respeta_veta: v === true })} />
                  <span className="md:hidden">Respeta veta</span>
                </label>
                <SelectorCantos valor={p.cantos} onChange={(cantos) => setPieza(i, { cantos })} />
                <Button variant="ghost" size="icon" aria-label="Quitar pieza"
                  className="text-muted-foreground hover:text-destructive"
                  onClick={() => setPiezas(piezas.filter((_, j) => j !== i))}>
                  <Trash2 />
                </Button>
              </div>
            </div>
          ))}
          <div className="flex flex-wrap gap-2 pt-2">
            <Button variant="outline" onClick={() => setPiezas([...piezas, nuevaPieza()])}>
              <Plus /> Agregar pieza
            </Button>
            <Button disabled={!puedeGenerar || optimizar.isPending || totalPiezas === 0}
              title={puedeGenerar ? undefined : 'Tu usuario no tiene permiso para generar cortes'} onClick={() => optimizar.mutate()}>
              {optimizar.isPending ? <Loader2 className="animate-spin" /> : <Scissors />}
              Optimizar cortes
            </Button>
            {res && (
              <Button variant="outline" onClick={() => window.print()}>
                <Printer /> Imprimir
              </Button>
            )}
          </div>
          {optimizar.isError && (
            <p className="flex items-center gap-2 text-sm text-destructive">
              <AlertTriangle className="size-4" /> {optimizar.error.message}
            </p>
          )}
        </CardContent>
      </Card>

      {res && (
        <section className="space-y-4">
          <div className="no-imprimir grid grid-cols-2 gap-3 lg:grid-cols-4">
            <Dato icono={Layers} titulo="Placas enteras" valor={String(res.placas_enteras)} />
            <Dato icono={Recycle} titulo="Retazos usados" valor={String(res.retazos_usados)} />
            <Dato icono={Scissors} titulo="Aprovechamiento" valor={`${res.aprovechamiento}%`} />
            <Dato icono={Ruler} titulo="Canto necesario" valor={`${res.metros_canto} m`} />
          </div>
          {res.no_colocadas.length > 0 && (
            <div className="flex items-start gap-2 rounded-lg border border-destructive/30 bg-destructive/10 p-3 text-sm text-destructive">
              <AlertTriangle className="mt-0.5 size-4 shrink-0" />
              No entran en la placa: {res.no_colocadas.map((p) => `${p.etiqueta || 'sin nombre'} ×${p.cantidad}`).join(', ')}
            </div>
          )}
          {res.placas.map((pl) => (
            <Card key={pl.indice} className="placa-hoja gap-3">
              <CardHeader className="flex flex-row flex-wrap items-center justify-between gap-2">
                <CardTitle className="flex items-center gap-2">
                  Placa {pl.indice}
                  {pl.retazo_id && <Badge variant="secondary">Retazo {pl.retazo_id}</Badge>}
                  <span className="text-sm font-normal text-muted-foreground">{pl.largo} × {pl.ancho} mm</span>
                </CardTitle>
                <div className="flex gap-2">
                  <Badge variant="outline">{pl.piezas.length} piezas</Badge>
                  <Badge>{pl.aprovechamiento}% usado</Badge>
                </div>
              </CardHeader>
              <CardContent>
                {/* En el celular la placa se puede deslizar de costado para leer las medidas. */}
                <div className="-mx-2 overflow-x-auto px-2 print:overflow-visible">
                  <div className="min-w-[640px] print:min-w-0">
                    <PlacaSVG placa={pl} piezas={piezas} tieneVeta={params.tiene_veta} />
                  </div>
                </div>
                <p className="mt-2 flex items-center gap-2 text-xs text-muted-foreground">
                  <span className="inline-block h-1 w-6 rounded-full bg-[oklch(0.52_0.2_28)]" />
                  Canto: la línea va del lado de adentro de la pieza que lo lleva, con la medida de ese lado.
                </p>
              </CardContent>
            </Card>
          ))}
        </section>
      )}
    </div>
  )
}
