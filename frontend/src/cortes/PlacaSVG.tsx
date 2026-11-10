import type { Pieza, PlacaUsada } from './tipos'

// Las piezas siempre son claras: el rojo del canto no cambia con el modo oscuro.
const ROJO_CANTO = 'oklch(0.52 0.2 28)'
const COLORES = ['#f6dfae', '#c9dcef', '#cfe5c6', '#f2cdc4', '#ddd3ee', '#f3d4b5', '#c4e4e0', '#ead1e3']

interface Props {
  placa: PlacaUsada
  piezas: Pieza[]
  tieneVeta: boolean
}

// Dibuja una placa con sus piezas. El eje X es el largo (sentido de la veta)
// y el origen de la API está abajo a la izquierda, así que se invierte Y.
export function PlacaSVG({ placa, piezas, tieneVeta }: Props) {
  const { largo: L, ancho: A } = placa
  const porId = new Map(piezas.map((p, i) => [p.id, { ...p, color: COLORES[i % COLORES.length] }]))
  const y = (yApi: number, alto: number) => A - yApi - alto
  const canto = Math.max(L, A) / 220

  return (
    <svg viewBox={`-20 -20 ${L + 40} ${A + 40}`} className="h-auto w-full" role="img"
      aria-label={`Placa ${placa.indice}`}>
      <defs>
        <pattern id="veta" width="400" height="40" patternUnits="userSpaceOnUse">
          <path d="M0 20 Q100 10 200 20 T400 20" fill="none" stroke="var(--placa-veta)" strokeWidth="3" />
        </pattern>
        <pattern id="sobrante" width="40" height="40" patternUnits="userSpaceOnUse" patternTransform="rotate(45)">
          <line x1="0" y1="0" x2="0" y2="40" stroke="var(--placa-veta)" strokeWidth="6" />
        </pattern>
      </defs>

      <rect x={0} y={0} width={L} height={A} fill="var(--placa-fondo)" stroke="var(--placa-borde)" strokeWidth={4} />
      {tieneVeta && <rect x={0} y={0} width={L} height={A} fill="url(#veta)" />}

      {(placa.sobrantes ?? []).map((s, i) => (
        <rect key={`s${i}`} x={s.x} y={y(s.y, s.ancho)} width={s.largo} height={s.ancho}
          fill="url(#sobrante)" stroke="var(--placa-borde)" strokeOpacity={0.5} strokeDasharray="20 12" strokeWidth={3}>
          <title>{`Sobrante ${s.largo} x ${s.ancho} mm`}</title>
        </rect>
      ))}

      {placa.piezas.map((c, i) => {
        const pz = porId.get(c.pieza_id)
        const top = y(c.y, c.ancho)
        const fuente = Math.max(24, Math.min(c.largo, c.ancho) / 5, 0)
        // L1/L2 son los lados largos de la pieza; si está rotada quedan verticales.
        // El canto se dibuja un poco hacia adentro de la pieza, con la medida
        // de ese lado, para que no se confunda con el borde de la pieza vecina.
        const cn = pz?.cantos
        const d = canto * 1.4
        const fCanto = Math.min(fuente * 0.75, 44)
        // La medida del canto va solo si no se pisa con el texto del centro.
        const fCentro = Math.min(fuente, 60)
        const anchoTexto = Math.max(c.etiqueta.length, `${c.largo} x ${c.ancho}`.length) * 0.55 * fCentro
        const margen = d + fCanto * 1.5
        // Las medidas de los lados verticales van arriba del texto si hay lugar;
        // si no, al medio, siempre que no lo pisen.
        const largoMedida = 3.2 * 0.6 * fCanto
        const libreArriba = c.ancho / 2 - 1.3 * fCentro - d
        const yVertical = libreArriba > largoMedida ? top + d + libreArriba / 2 : top + c.ancho / 2
        const entraVertical = libreArriba > largoMedida || (c.largo - anchoTexto) / 2 > margen
        const entraHorizontal = (c.ancho - 2.4 * fCentro) / 2 > margen
        type Lado = { x1: number; y1: number; x2: number; y2: number; tx: number; ty: number; rot: number; medida: number }
        const abajo: Lado = { x1: c.x + d, y1: top + c.ancho - d, x2: c.x + c.largo - d, y2: top + c.ancho - d,
          tx: c.x + c.largo / 2, ty: top + c.ancho - d - fCanto * 0.9, rot: 0, medida: c.largo }
        const arriba: Lado = { x1: c.x + d, y1: top + d, x2: c.x + c.largo - d, y2: top + d,
          tx: c.x + c.largo / 2, ty: top + d + fCanto * 0.9, rot: 0, medida: c.largo }
        const izq: Lado = { x1: c.x + d, y1: top + d, x2: c.x + d, y2: top + c.ancho - d,
          tx: c.x + d + fCanto * 0.9, ty: yVertical, rot: -90, medida: c.ancho }
        const der: Lado = { x1: c.x + c.largo - d, y1: top + d, x2: c.x + c.largo - d, y2: top + c.ancho - d,
          tx: c.x + c.largo - d - fCanto * 0.9, ty: yVertical, rot: 90, medida: c.ancho }
        const [l1, l2, a1, a2] = !c.rotada ? [abajo, arriba, izq, der] : [izq, der, abajo, arriba]
        const lados = cn ? ([[cn.l1, l1], [cn.l2, l2], [cn.a1, a1], [cn.a2, a2]] as const).filter(([si]) => si).map(([, l]) => l) : []
        const conCanto = cn ? [cn.l1 && 'L1', cn.l2 && 'L2', cn.a1 && 'A1', cn.a2 && 'A2'].filter(Boolean).join(', ') : ''
        return (
          <g key={i}>
            <rect x={c.x} y={top} width={c.largo} height={c.ancho} fill={pz?.color ?? '#e7e0d6'}
              fillOpacity={0.95} stroke="#3b2a1e" strokeWidth={2}>
              <title>{`${c.etiqueta}: ${c.largo} x ${c.ancho} mm${c.rotada ? ' (rotada)' : ''}${conCanto ? ` · canto en ${conCanto}` : ''}`}</title>
            </rect>
            {lados.map((l, j) => (
              <g key={j} pointerEvents="none">
                <line x1={l.x1} y1={l.y1} x2={l.x2} y2={l.y2} stroke={ROJO_CANTO} strokeWidth={canto} strokeLinecap="round" />
                {(l.rot ? entraVertical : entraHorizontal) && (
                  <text x={l.tx} y={l.ty} textAnchor="middle" dominantBaseline="middle" fontSize={fCanto}
                    fontWeight={600} fill={ROJO_CANTO} transform={l.rot ? `rotate(${l.rot} ${l.tx} ${l.ty})` : undefined}>
                    {l.medida}
                  </text>
                )}
              </g>
            ))}
            <text x={c.x + c.largo / 2} y={top + c.ancho / 2} textAnchor="middle" dominantBaseline="middle"
              fontSize={fCentro} fill="#2a1d14" pointerEvents="none">
              <tspan x={c.x + c.largo / 2} dy="-0.6em">{c.etiqueta}</tspan>
              <tspan x={c.x + c.largo / 2} dy="1.2em">{`${c.largo} x ${c.ancho}`}</tspan>
            </text>
          </g>
        )
      })}
    </svg>
  )
}
