import { useState } from 'react'
import { useMutation } from '@tanstack/react-query'
import { postJSON } from '../api/cliente'
import { PlacaSVG } from './PlacaSVG'
import type { Parametros, Pieza, Resultado } from './tipos'

let siguienteId = 3
const nuevaPieza = (): Pieza => ({
  id: String(siguienteId++), etiqueta: '', largo: 0, ancho: 0, cantidad: 1, respeta_veta: true,
  cantos: { l1: false, l2: false, a1: false, a2: false },
})

const ejemplo: Pieza[] = [
  { id: '1', etiqueta: 'Lateral', largo: 720, ancho: 560, cantidad: 2, respeta_veta: true,
    cantos: { l1: true, l2: false, a1: false, a2: false } },
  { id: '2', etiqueta: 'Puerta', largo: 716, ancho: 396, cantidad: 2, respeta_veta: true,
    cantos: { l1: true, l2: true, a1: true, a2: true } },
]

const input = 'w-full rounded border border-slate-300 px-2 py-1 text-sm'

export function Optimizador() {
  const [params, setParams] = useState<Parametros>({
    placa_largo: 2600, placa_ancho: 1830, tiene_veta: true, kerf: 3, refilado: 0,
    sobrante_min_largo: 300, sobrante_min_ancho: 150,
  })
  const [piezas, setPiezas] = useState<Pieza[]>(ejemplo)

  const optimizar = useMutation({
    mutationFn: () =>
      postJSON<Resultado>('/api/v1/cortes/optimizar', {
        parametros: params,
        piezas: piezas.filter((p) => p.largo > 0 && p.ancho > 0),
      }),
  })

  const setPieza = (i: number, cambios: Partial<Pieza>) =>
    setPiezas((ps) => ps.map((p, j) => (j === i ? { ...p, ...cambios } : p)))
  const num = (v: string) => Math.max(0, Number(v) || 0)
  const res = optimizar.data

  return (
    <div className="space-y-6">
      <section className="no-imprimir rounded-lg bg-white p-4 shadow-sm">
        <h2 className="mb-3 font-semibold">Placa</h2>
        <div className="grid grid-cols-2 gap-3 sm:grid-cols-5">
          {([
            ['placa_largo', 'Largo (mm)'], ['placa_ancho', 'Ancho (mm)'],
            ['kerf', 'Sierra (mm)'], ['refilado', 'Refilado (mm)'],
          ] as const).map(([k, etiqueta]) => (
            <label key={k} className="text-sm">
              {etiqueta}
              <input type="number" className={input} value={params[k]}
                onChange={(e) => setParams({ ...params, [k]: num(e.target.value) })} />
            </label>
          ))}
          <label className="flex items-center gap-2 self-end pb-1 text-sm">
            <input type="checkbox" checked={params.tiene_veta}
              onChange={(e) => setParams({ ...params, tiene_veta: e.target.checked })} />
            Tiene veta
          </label>
        </div>
      </section>

      <section className="no-imprimir rounded-lg bg-white p-4 shadow-sm">
        <h2 className="mb-3 font-semibold">Piezas</h2>
        <div className="overflow-x-auto">
          <table className="w-full min-w-[640px] text-sm">
            <thead className="text-left text-slate-500">
              <tr>
                <th className="pb-2">Nombre</th><th>Largo</th><th>Ancho</th><th>Cant.</th>
                <th title="No se puede rotar">Veta</th>
                <th title="Cantos: L1 L2 lados largos, A1 A2 lados anchos">Cantos L1 L2 A1 A2</th><th />
              </tr>
            </thead>
            <tbody>
              {piezas.map((p, i) => (
                <tr key={p.id} className="border-t border-slate-100">
                  <td className="py-1 pr-2"><input className={input} value={p.etiqueta}
                    onChange={(e) => setPieza(i, { etiqueta: e.target.value })} /></td>
                  {(['largo', 'ancho', 'cantidad'] as const).map((k) => (
                    <td key={k} className="pr-2"><input type="number" className={`${input} w-20`} value={p[k]}
                      onChange={(e) => setPieza(i, { [k]: num(e.target.value) })} /></td>
                  ))}
                  <td><input type="checkbox" checked={p.respeta_veta}
                    onChange={(e) => setPieza(i, { respeta_veta: e.target.checked })} /></td>
                  <td className="space-x-3">
                    {(['l1', 'l2', 'a1', 'a2'] as const).map((k) => (
                      <input key={k} type="checkbox" title={k.toUpperCase()} checked={p.cantos[k]}
                        onChange={(e) => setPieza(i, { cantos: { ...p.cantos, [k]: e.target.checked } })} />
                    ))}
                  </td>
                  <td><button className="text-red-600" onClick={() => setPiezas(piezas.filter((_, j) => j !== i))}>
                    Quitar</button></td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
        <div className="mt-3 flex flex-wrap gap-2">
          <button className="rounded border border-slate-300 px-3 py-1.5 text-sm"
            onClick={() => setPiezas([...piezas, nuevaPieza()])}>Agregar pieza</button>
          <button className="rounded bg-amber-700 px-3 py-1.5 text-sm font-medium text-white disabled:opacity-50"
            disabled={optimizar.isPending} onClick={() => optimizar.mutate()}>
            {optimizar.isPending ? 'Calculando…' : 'Optimizar cortes'}</button>
          {res && <button className="rounded border border-slate-300 px-3 py-1.5 text-sm"
            onClick={() => window.print()}>Imprimir</button>}
        </div>
        {optimizar.isError && <p className="mt-2 text-sm text-red-600">{optimizar.error.message}</p>}
      </section>

      {res && (
        <section className="space-y-4">
          <div className="no-imprimir grid grid-cols-2 gap-3 sm:grid-cols-4">
            {[
              ['Placas enteras', res.placas_enteras], ['Retazos usados', res.retazos_usados],
              ['Aprovechamiento', `${res.aprovechamiento}%`], ['Canto', `${res.metros_canto} m`],
            ].map(([k, v]) => (
              <div key={k} className="rounded-lg bg-white p-3 shadow-sm">
                <div className="text-xs text-slate-500">{k}</div>
                <div className="text-xl font-semibold">{v}</div>
              </div>
            ))}
          </div>
          {res.no_colocadas.length > 0 && (
            <p className="rounded bg-red-50 p-3 text-sm text-red-700">
              No entran en la placa: {res.no_colocadas.map((p) => `${p.etiqueta} x${p.cantidad}`).join(', ')}
            </p>
          )}
          {res.placas.map((pl) => (
            <div key={pl.indice} className="placa-hoja rounded-lg bg-white p-3 shadow-sm">
              <div className="mb-2 flex justify-between text-sm">
                <strong>Placa {pl.indice}{pl.retazo_id ? ` (retazo ${pl.retazo_id})` : ''}: {pl.largo} x {pl.ancho} mm</strong>
                <span>{pl.piezas.length} piezas · {pl.aprovechamiento}% usado</span>
              </div>
              <PlacaSVG placa={pl} piezas={piezas} tieneVeta={params.tiene_veta} />
            </div>
          ))}
        </section>
      )}
    </div>
  )
}
