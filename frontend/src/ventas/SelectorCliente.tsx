import { useState, type FormEvent } from 'react'
import { Loader2, Search, UserPlus, X } from 'lucide-react'
import { Button } from '@/components/ui/button'
import {
  Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle,
} from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { usePermiso } from '@/auth/sesion'
import { useClientes, useGuardarCliente, type Cliente } from './api'

function Campo({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <label className="grid gap-1.5 text-sm font-medium">
      {label}
      {children}
    </label>
  )
}

export function DialogoCliente({ inicial, onClose, onGuardado }: {
  inicial: string; onClose: () => void; onGuardado: (c: Cliente) => void
}) {
  const guardar = useGuardarCliente()
  const [c, setC] = useState({ nombre: inicial, documento: '', telefono: '', email: '', direccion: '' })
  const set = (k: keyof typeof c) => (e: React.ChangeEvent<HTMLInputElement>) => setC({ ...c, [k]: e.target.value })
  const enviar = (e: FormEvent) => {
    e.preventDefault()
    guardar.mutate(c, { onSuccess: onGuardado })
  }
  return (
    <Dialog open onOpenChange={(o) => !o && onClose()}>
      <DialogContent>
        <form onSubmit={enviar} className="grid gap-4">
          <DialogHeader>
            <DialogTitle>Cliente nuevo</DialogTitle>
            <DialogDescription>El DNI o CUIT y el teléfono salen en la boleta.</DialogDescription>
          </DialogHeader>
          <Campo label="Nombre y apellido"><Input value={c.nombre} onChange={set('nombre')} autoFocus required /></Campo>
          <div className="grid gap-4 sm:grid-cols-2">
            <Campo label="DNI o CUIT"><Input value={c.documento} onChange={set('documento')} inputMode="numeric" /></Campo>
            <Campo label="Teléfono"><Input value={c.telefono} onChange={set('telefono')} inputMode="tel" /></Campo>
          </div>
          <Campo label="Email"><Input type="email" value={c.email} onChange={set('email')} /></Campo>
          <Campo label="Dirección"><Input value={c.direccion} onChange={set('direccion')} /></Campo>
          {guardar.error && <p className="text-sm text-destructive">{guardar.error.message}</p>}
          <DialogFooter>
            <Button type="button" variant="outline" onClick={onClose}>Cancelar</Button>
            <Button type="submit" disabled={guardar.isPending}>
              {guardar.isPending && <Loader2 className="animate-spin" />} Guardar cliente
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}

export function SelectorCliente({ valor, onChange }: { valor: Cliente | null; onChange: (c: Cliente | null) => void }) {
  const [buscar, setBuscar] = useState('')
  const [nuevo, setNuevo] = useState(false)
  const puedeCrear = usePermiso('clientes.editar')
  const clientes = useClientes(buscar)

  if (valor) {
    return (
      <div className="flex items-start justify-between gap-3 rounded-lg border bg-muted/30 p-3">
        <div className="min-w-0 text-sm">
          <div className="font-medium">{valor.nombre}</div>
          <div className="text-muted-foreground">
            {[valor.documento && `DNI/CUIT ${valor.documento}`, valor.telefono, valor.direccion].filter(Boolean).join(' · ') || 'Sin datos de contacto'}
          </div>
        </div>
        <Button variant="ghost" size="icon" aria-label="Cambiar cliente" onClick={() => onChange(null)}><X /></Button>
      </div>
    )
  }

  const lista = buscar.trim() ? (clientes.data ?? []).slice(0, 6) : []
  return (
    <div className="space-y-2">
      <div className="flex gap-2">
        <div className="relative flex-1">
          <Search className="pointer-events-none absolute top-1/2 left-2.5 size-4 -translate-y-1/2 text-muted-foreground" />
          <Input className="pl-8" placeholder="Buscar cliente por nombre, DNI o teléfono" value={buscar}
            onChange={(e) => setBuscar(e.target.value)} aria-label="Buscar cliente" />
        </div>
        {puedeCrear && (
          <Button type="button" variant="outline" onClick={() => setNuevo(true)}><UserPlus /> <span className="hidden sm:inline">Nuevo</span></Button>
        )}
      </div>
      {lista.length > 0 && (
        <div className="divide-y rounded-lg border">
          {lista.map((c) => (
            <button key={c.id} type="button" onClick={() => onChange(c)}
              className="flex w-full items-center justify-between gap-2 px-3 py-2 text-left text-sm hover:bg-accent">
              <span className="font-medium">{c.nombre}</span>
              <span className="truncate text-muted-foreground">{c.telefono || c.documento}</span>
            </button>
          ))}
        </div>
      )}
      {buscar.trim() && clientes.isSuccess && lista.length === 0 && (
        <p className="text-sm text-muted-foreground">No hay clientes con ese nombre.{puedeCrear && ' Podés crearlo con “Nuevo”.'}</p>
      )}
      {nuevo && (
        <DialogoCliente inicial={buscar} onClose={() => setNuevo(false)}
          onGuardado={(c) => { setNuevo(false); onChange(c) }} />
      )}
    </div>
  )
}
