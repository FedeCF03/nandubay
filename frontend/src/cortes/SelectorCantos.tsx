import { cn } from '@/lib/utils'
import type { Cantos } from './tipos'

// Mini pieza con sus cuatro lados: se toca un lado para ponerle o sacarle canto.
// L1 abajo y L2 arriba (lados largos), A1 izquierda y A2 derecha (lados anchos).
export function SelectorCantos({ valor, onChange }: { valor: Cantos; onChange: (c: Cantos) => void }) {
  const lado = (k: keyof Cantos, clase: string, nombre: string) => (
    <button
      type="button"
      aria-label={`Canto ${nombre}`}
      aria-pressed={valor[k]}
      title={`${k.toUpperCase()}: ${nombre}`}
      onClick={() => onChange({ ...valor, [k]: !valor[k] })}
      className={cn('absolute rounded-full transition-colors',
        valor[k] ? 'bg-[var(--canto)]' : 'bg-border hover:bg-muted-foreground/40', clase)}
    />
  )
  return (
    <div className="relative h-9 w-14 shrink-0">
      <div className="absolute inset-[5px] rounded-sm bg-muted" />
      {lado('l2', 'inset-x-1.5 top-0 h-1.5', 'largo de arriba')}
      {lado('l1', 'inset-x-1.5 bottom-0 h-1.5', 'largo de abajo')}
      {lado('a1', 'inset-y-1.5 left-0 w-1.5', 'ancho izquierdo')}
      {lado('a2', 'inset-y-1.5 right-0 w-1.5', 'ancho derecho')}
    </div>
  )
}
