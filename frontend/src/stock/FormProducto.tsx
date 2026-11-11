import { useState, type FormEvent } from 'react'
import { Loader2 } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import {
  Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle,
} from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { nombreTipo, useCategorias, useGuardarProducto, type Producto, type TipoProducto } from './api'

const vacio: Partial<Producto> = {
  tipo: 'placa', nombre: '', sku: '', marca: '', color: '', costo: 0, precio_venta: 0, stock_minimo: 0,
  largo_mm: 2600, ancho_mm: 1830, espesor_mm: 18, tiene_veta: false, refilado_mm: 0, activo: true, categoria_id: null,
}

function Campo({ label, children, className }: { label: string; children: React.ReactNode; className?: string }) {
  return (
    // El label envuelve al campo para que quede asociado (accesibilidad).
    <label className={`grid gap-1.5 ${className ?? ''}`}>
      <span className="text-xs font-medium text-muted-foreground">{label}</span>
      {children}
    </label>
  )
}

export function FormProducto({ producto, onClose }: { producto: Partial<Producto> | null; onClose: () => void }) {
  const [p, setP] = useState<Partial<Producto>>(producto?.id ? producto : vacio)
  const guardar = useGuardarProducto()
  const categorias = useCategorias().data ?? []
  const editando = !!producto?.id
  const set = (c: Partial<Producto>) => setP((v) => ({ ...v, ...c }))
  const num = (k: keyof Producto) => ({
    type: 'number', inputMode: 'decimal' as const, min: 0, step: 'any',
    value: (p[k] as number | undefined) ?? '',
    onChange: (e: React.ChangeEvent<HTMLInputElement>) => set({ [k]: e.target.value === '' ? 0 : Number(e.target.value) }),
  })

  const enviar = (e: FormEvent) => {
    e.preventDefault()
    guardar.mutate(p, { onSuccess: onClose })
  }

  return (
    <Dialog open onOpenChange={(o) => !o && onClose()}>
      <DialogContent className="max-h-[90svh] overflow-y-auto sm:max-w-lg">
        <form onSubmit={enviar} className="grid gap-4">
          <DialogHeader>
            <DialogTitle>{editando ? 'Editar producto' : 'Nuevo producto'}</DialogTitle>
            <DialogDescription>
              {editando ? 'El tipo no se puede cambiar.' : 'Placas, cantos, herrajes o cualquier otro insumo.'}
            </DialogDescription>
          </DialogHeader>

          <div className="grid grid-cols-2 gap-3">
            <Campo label="Tipo">
              <Select value={p.tipo} disabled={editando} onValueChange={(v) => set({ tipo: v as TipoProducto })}>
                <SelectTrigger className="w-full"><SelectValue /></SelectTrigger>
                <SelectContent>
                  {(Object.keys(nombreTipo) as TipoProducto[]).map((t) => (
                    <SelectItem key={t} value={t}>{nombreTipo[t]}</SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </Campo>
            <Campo label="Código (opcional)">
              <Input value={p.sku} onChange={(e) => set({ sku: e.target.value })} placeholder="MEL-BL-18" />
            </Campo>
            <Campo label="Nombre" className="col-span-2">
              <Input required value={p.nombre} onChange={(e) => set({ nombre: e.target.value })}
                placeholder={p.tipo === 'placa' ? 'Melamina Blanca 18 mm' : p.tipo === 'herraje' ? 'Bisagra cazoleta 35 mm' : ''} />
            </Campo>
            <Campo label="Categoría" className="col-span-2">
              <Select value={p.categoria_id ? String(p.categoria_id) : 'ninguna'}
                onValueChange={(v) => set({ categoria_id: v === 'ninguna' ? null : Number(v) })}>
                <SelectTrigger className="w-full"><SelectValue /></SelectTrigger>
                <SelectContent>
                  <SelectItem value="ninguna">Sin categoría</SelectItem>
                  {categorias.map((c) => <SelectItem key={c.id} value={String(c.id)}>{c.nombre}</SelectItem>)}
                </SelectContent>
              </Select>
            </Campo>
            <Campo label="Marca"><Input value={p.marca} onChange={(e) => set({ marca: e.target.value })} /></Campo>
            <Campo label="Color"><Input value={p.color} onChange={(e) => set({ color: e.target.value })} /></Campo>
          </div>

          {p.tipo === 'placa' && (
            <div className="grid grid-cols-2 gap-3 rounded-lg border bg-muted/40 p-3 sm:grid-cols-4">
              <Campo label="Largo (mm)"><Input {...num('largo_mm')} required /></Campo>
              <Campo label="Ancho (mm)"><Input {...num('ancho_mm')} required /></Campo>
              <Campo label="Espesor (mm)"><Input {...num('espesor_mm')} required /></Campo>
              <Campo label="Refilado (mm)"><Input {...num('refilado_mm')} /></Campo>
              <label className="col-span-2 flex items-center gap-2 text-sm sm:col-span-4">
                <Checkbox checked={p.tiene_veta} onCheckedChange={(v) => set({ tiene_veta: v === true })} />
                Tiene veta (las piezas no se rotan al cortar)
              </label>
            </div>
          )}
          {p.tipo === 'canto' && (
            <div className="grid grid-cols-2 gap-3 rounded-lg border bg-muted/40 p-3">
              <Campo label="Ancho (mm)"><Input {...num('canto_ancho_mm')} /></Campo>
              <Campo label="Espesor (mm)"><Input {...num('canto_espesor_mm')} /></Campo>
            </div>
          )}

          <div className="grid grid-cols-3 gap-3">
            <Campo label="Costo ($)"><Input {...num('costo')} /></Campo>
            <Campo label="Precio de venta ($)"><Input {...num('precio_venta')} /></Campo>
            <Campo label={`Stock mínimo (${p.tipo === 'placa' ? 'placas' : p.tipo === 'canto' ? 'metros' : 'unidades'})`}>
              <Input {...num('stock_minimo')} />
            </Campo>
          </div>

          {editando && (
            <label className="flex items-center gap-2 text-sm">
              <Checkbox checked={!p.activo} onCheckedChange={(v) => set({ activo: v !== true })} />
              Dar de baja (deja de aparecer en las listas)
            </label>
          )}

          {guardar.isError && <p role="alert" className="text-sm text-destructive">{guardar.error.message}</p>}
          <DialogFooter>
            <Button type="button" variant="outline" onClick={onClose}>Cancelar</Button>
            <Button type="submit" disabled={guardar.isPending}>
              {guardar.isPending && <Loader2 className="animate-spin" />} Guardar
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}
