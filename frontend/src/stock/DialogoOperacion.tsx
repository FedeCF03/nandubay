import { useState, type FormEvent } from 'react'
import { Loader2 } from 'lucide-react'
import { Button } from '@/components/ui/button'
import {
  Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle,
} from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Textarea } from '@/components/ui/textarea'
import { useUsuario } from '@/auth/sesion'
import {
  fmtNumero, unidadCorta, useDepositos, useOperacion, type ProductoConStock, type TipoOperacion,
} from './api'

const textos: Record<TipoOperacion, { titulo: string; desc: string; cantidad: string; boton: string }> = {
  ingresos: { titulo: 'Ingreso de mercadería', desc: 'Suma stock al depósito, por ejemplo al recibir una compra.', cantidad: 'Cantidad que entra', boton: 'Registrar ingreso' },
  ajustes: { titulo: 'Ajuste por conteo', desc: 'Cargá lo que contaste y se corrige la diferencia.', cantidad: 'Stock contado', boton: 'Ajustar' },
  traspasos: { titulo: 'Traspaso entre depósitos', desc: 'Pasá mercadería de un depósito a otro, aunque sea de otra sucursal.', cantidad: 'Cantidad a pasar', boton: 'Traspasar' },
}

export function DialogoOperacion({ tipo, producto, onClose }: {
  tipo: TipoOperacion; producto: ProductoConStock; onClose: () => void
}) {
  const u = useUsuario()
  const depositos = useDepositos().data ?? []
  const operar = useOperacion()
  // Por defecto, un depósito de la sucursal actual.
  const propio = depositos.find((d) => d.sucursal_id === u.sucursal_id) ?? depositos[0]
  const [deposito, setDeposito] = useState<string>('')
  const [destino, setDestino] = useState<string>('')
  const [cantidad, setCantidad] = useState('')
  const [costo, setCosto] = useState(producto.costo ? String(producto.costo) : '')
  const [nota, setNota] = useState('')
  const origenID = deposito || (propio ? String(propio.id) : '')
  const enDeposito = producto.por_deposito[origenID] ?? 0
  const t = textos[tipo]
  const u2 = unidadCorta[producto.unidad] ?? producto.unidad

  const enviar = (e: FormEvent) => {
    e.preventDefault()
    operar.mutate({
      tipo,
      datos: {
        producto_id: producto.id,
        deposito_id: Number(origenID),
        destino_id: destino ? Number(destino) : undefined,
        cantidad: Number(cantidad.replace(',', '.')),
        costo_unitario: tipo === 'ingresos' && costo !== '' ? Number(costo.replace(',', '.')) : undefined,
        nota,
      },
    }, { onSuccess: onClose })
  }

  const selector = (valor: string, onChange: (v: string) => void, excluir?: string) => (
    <Select value={valor} onValueChange={onChange}>
      <SelectTrigger className="w-full"><SelectValue placeholder="Elegí un depósito" /></SelectTrigger>
      <SelectContent>
        {depositos.filter((d) => String(d.id) !== excluir).map((d) => (
          <SelectItem key={d.id} value={String(d.id)}>
            {d.sucursal} · {d.nombre}
          </SelectItem>
        ))}
      </SelectContent>
    </Select>
  )

  return (
    <Dialog open onOpenChange={(o) => !o && onClose()}>
      <DialogContent className="sm:max-w-md">
        <form onSubmit={enviar} className="grid min-w-0 gap-4">
          <DialogHeader>
            <DialogTitle>{t.titulo}</DialogTitle>
            <DialogDescription>{producto.nombre}. {t.desc}</DialogDescription>
          </DialogHeader>

          <div className="grid gap-3">
            <div className="grid gap-1.5">
              <Label className="text-xs text-muted-foreground">{tipo === 'traspasos' ? 'Desde' : 'Depósito'}</Label>
              {selector(origenID, setDeposito)}
            </div>
            {tipo === 'traspasos' && (
              <>
                <div className="grid gap-1.5">
                  <Label className="text-xs text-muted-foreground">Hacia</Label>
                  {selector(destino, setDestino, origenID)}
                </div>
              </>
            )}
          </div>
          <p className="-mt-2 text-xs text-muted-foreground">
            Hay {fmtNumero(enDeposito)} {u2} en {tipo === 'traspasos' ? 'el depósito de origen' : 'ese depósito'}.
          </p>

          <div className="grid grid-cols-2 gap-3">
            <div className="grid gap-1.5">
              <Label htmlFor="cantidad" className="text-xs text-muted-foreground">{t.cantidad} ({u2})</Label>
              <Input id="cantidad" required inputMode="decimal" autoFocus value={cantidad}
                onChange={(e) => setCantidad(e.target.value)} />
            </div>
            {tipo === 'ingresos' && (
              <div className="grid gap-1.5">
                <Label htmlFor="costo" className="text-xs text-muted-foreground">Costo unitario ($)</Label>
                <Input id="costo" inputMode="decimal" value={costo} onChange={(e) => setCosto(e.target.value)} />
              </div>
            )}
          </div>
          <div className="grid gap-1.5">
            <Label htmlFor="nota" className="text-xs text-muted-foreground">
              {tipo === 'ajustes' ? 'Motivo (obligatorio)' : 'Nota (opcional)'}
            </Label>
            <Textarea id="nota" rows={2} value={nota} required={tipo === 'ajustes'}
              placeholder={tipo === 'ajustes' ? 'Ej: 2 placas rotas en el traslado' : tipo === 'ingresos' ? 'Ej: Factura 0001-00012345 de Maderera Sur' : ''}
              onChange={(e) => setNota(e.target.value)} />
          </div>

          {operar.isError && <p role="alert" className="text-sm text-destructive">{operar.error.message}</p>}
          <DialogFooter>
            <Button type="button" variant="outline" onClick={onClose}>Cancelar</Button>
            <Button type="submit" disabled={operar.isPending || !origenID || (tipo === 'traspasos' && !destino)}>
              {operar.isPending && <Loader2 className="animate-spin" />} {t.boton}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}
