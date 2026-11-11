import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { cn } from '@/lib/utils'
import { nombreVeta, type Veta } from './veta'

// IconoVeta dibuja la pieza (largo horizontal) con las vetas en la dirección elegida.
export function IconoVeta({ veta, className }: { veta: Veta; className?: string }) {
  return (
    <svg viewBox="0 0 24 16" className={cn('h-3.5 w-5 shrink-0', className)} aria-hidden>
      <rect x="1" y="1" width="22" height="14" rx="1.5" fill="none" stroke="currentColor" strokeWidth="1.5" />
      {veta === 'h' && [5, 8, 11].map((y) => <line key={y} x1="4" x2="20" y1={y} y2={y} stroke="currentColor" strokeWidth="1.2" />)}
      {veta === 'v' && [7, 12, 17].map((x) => <line key={x} x1={x} x2={x} y1="4" y2="12" stroke="currentColor" strokeWidth="1.2" />)}
    </svg>
  )
}

export function SelectorVeta({ valor, onChange, disabled, className, etiqueta = 'Veta' }: {
  valor: Veta
  onChange: (v: Veta) => void
  disabled?: boolean
  className?: string
  etiqueta?: string
}) {
  return (
    <Select value={disabled ? 'no' : valor} onValueChange={(v) => onChange(v as Veta)} disabled={disabled}>
      <SelectTrigger size="sm" className={cn('w-32', className)} aria-label={etiqueta}
        title="Dirección de la veta en la pieza: horizontal va a lo largo, vertical a lo ancho">
        <SelectValue />
      </SelectTrigger>
      <SelectContent>
        {(['no', 'h', 'v'] as const).map((v) => (
          <SelectItem key={v} value={v}><IconoVeta veta={v} /> {nombreVeta[v]}</SelectItem>
        ))}
      </SelectContent>
    </Select>
  )
}
