import type { Pieza, PlacaUsada } from './tipos'

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
        const ladosLargoHorizontales = !c.rotada
        const cn = pz?.cantos
        const lineas: [number, number, number, number][] = []
        if (cn) {
          const abajo: [number, number, number, number] = [c.x, top + c.ancho, c.x + c.largo, top + c.ancho]
          const arriba: [number, number, number, number] = [c.x, top, c.x + c.largo, top]
          const izq: [number, number, number, number] = [c.x, top, c.x, top + c.ancho]
          const der: [number, number, number, number] = [c.x + c.largo, top, c.x + c.largo, top + c.ancho]
          const [l1, l2, a1, a2] = ladosLargoHorizontales ? [abajo, arriba, izq, der] : [izq, der, abajo, arriba]
          if (cn.l1) lineas.push(l1)
          if (cn.l2) lineas.push(l2)
          if (cn.a1) lineas.push(a1)
          if (cn.a2) lineas.push(a2)
        }
        return (
          <g key={i}>
            <rect x={c.x} y={top} width={c.largo} height={c.ancho} fill={pz?.color ?? '#e7e0d6'}
              fillOpacity={0.95} stroke="#3b2a1e" strokeWidth={2}>
              <title>{`${c.etiqueta}: ${c.largo} x ${c.ancho} mm${c.rotada ? ' (rotada)' : ''}`}</title>
            </rect>
            {lineas.map(([x1, y1, x2, y2], j) => (
              <line key={j} x1={x1} y1={y1} x2={x2} y2={y2} stroke="var(--canto)" strokeWidth={canto} />
            ))}
            <text x={c.x + c.largo / 2} y={top + c.ancho / 2} textAnchor="middle" dominantBaseline="middle"
              fontSize={Math.min(fuente, 60)} fill="#2a1d14" pointerEvents="none">
              <tspan x={c.x + c.largo / 2} dy="-0.6em">{c.etiqueta}</tspan>
              <tspan x={c.x + c.largo / 2} dy="1.2em">{`${c.largo} x ${c.ancho}`}</tspan>
            </text>
          </g>
        )
      })}
    </svg>
  )
}
