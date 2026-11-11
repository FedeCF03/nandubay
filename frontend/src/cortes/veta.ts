// Veta de una pieza: sin veta (se puede girar), horizontal (a lo largo de la
// pieza) o vertical (a lo ancho).
export type Veta = 'no' | 'h' | 'v'

export const vetaDe = (p: { respeta_veta: boolean; veta_vertical?: boolean }): Veta =>
  !p.respeta_veta ? 'no' : p.veta_vertical ? 'v' : 'h'

export const conVeta = (v: Veta) => ({ respeta_veta: v !== 'no', veta_vertical: v === 'v' })

export const nombreVeta: Record<Veta, string> = { no: 'Sin veta', h: 'Horizontal', v: 'Vertical' }
