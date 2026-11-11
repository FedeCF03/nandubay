import { useState, type FormEvent } from 'react'
import { Link } from 'wouter'
import { Loader2, MoreHorizontal, Pencil, Plus, Tags, Trash2 } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Card } from '@/components/ui/card'
import {
  Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle,
} from '@/components/ui/dialog'
import {
  DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { Input } from '@/components/ui/input'
import { Skeleton } from '@/components/ui/skeleton'
import { Textarea } from '@/components/ui/textarea'
import { usePermiso } from '@/auth/sesion'
import { useBorrarCategoria, useCategorias, useGuardarCategoria, type Categoria } from '@/stock/api'

// Ideas para arrancar cuando todavía no hay ninguna.
const SUGERIDAS = ['Placas', 'Cantos', 'Bisagras', 'Guías', 'Manijas', 'Patas', 'Tornillería']

const productosTexto = (n: number) => (n === 1 ? '1 producto' : `${n} productos`)

export default function Categorias() {
  const categorias = useCategorias()
  const edita = usePermiso('productos.editar')
  const [editar, setEditar] = useState<Partial<Categoria> | null>(null)
  const [borrar, setBorrar] = useState<Categoria | null>(null)
  const guardar = useGuardarCategoria()
  const lista = categorias.data ?? []

  const acciones = (c: Categoria) => edita && (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button variant="ghost" size="icon" aria-label={`Acciones de ${c.nombre}`}><MoreHorizontal /></Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end">
        <DropdownMenuItem onSelect={() => setEditar(c)}><Pencil /> Editar</DropdownMenuItem>
        <DropdownMenuItem variant="destructive" onSelect={() => setBorrar(c)}><Trash2 /> Borrar</DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  )

  return (
    <div className="space-y-5">
      <div className="flex flex-wrap items-end justify-between gap-3">
        <div>
          <h2 className="text-2xl font-semibold tracking-tight">Categorías de productos</h2>
          <p className="text-muted-foreground">Agrupá los productos del stock para encontrarlos y filtrarlos más rápido.</p>
        </div>
        {edita && <Button onClick={() => setEditar({ nombre: '', descripcion: '' })}><Plus /> Nueva categoría</Button>}
      </div>

      {categorias.isPending ? (
        <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
          {Array.from({ length: 3 }, (_, i) => <Skeleton key={i} className="h-24" />)}
        </div>
      ) : categorias.isError ? (
        <p role="alert" className="text-sm text-destructive">{categorias.error.message}</p>
      ) : lista.length === 0 ? (
        <div className="flex flex-col items-center gap-3 rounded-xl border border-dashed px-4 py-14 text-center">
          <Tags className="size-10 text-muted-foreground" />
          <p className="font-medium">Todavía no hay categorías.</p>
          {edita && (
            <>
              <p className="text-sm text-muted-foreground">Podés crear una a mano o empezar con alguna de estas:</p>
              <div className="flex max-w-lg flex-wrap justify-center gap-2">
                {SUGERIDAS.map((nombre) => (
                  <Button key={nombre} variant="outline" size="sm" disabled={guardar.isPending}
                    onClick={() => guardar.mutate({ nombre, descripcion: '' })}>
                    <Plus /> {nombre}
                  </Button>
                ))}
              </div>
            </>
          )}
        </div>
      ) : (
        <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
          {lista.map((c) => (
            <Card key={c.id} className="gap-1 px-4 py-3">
              <div className="flex items-start justify-between gap-2">
                <div className="min-w-0">
                  <div className="truncate font-medium">{c.nombre}</div>
                  <Link href={`/stock?categoria=${c.id}`} className="text-xs text-primary hover:underline">
                    {productosTexto(c.productos)}
                  </Link>
                </div>
                {acciones(c)}
              </div>
              {c.descripcion && <p className="line-clamp-2 text-sm text-muted-foreground">{c.descripcion}</p>}
            </Card>
          ))}
        </div>
      )}

      {editar && <FormCategoria categoria={editar} onClose={() => setEditar(null)} />}
      {borrar && <BorrarCategoria categoria={borrar} onClose={() => setBorrar(null)} />}
    </div>
  )
}

function FormCategoria({ categoria, onClose }: { categoria: Partial<Categoria>; onClose: () => void }) {
  const [c, setC] = useState(categoria)
  const guardar = useGuardarCategoria()
  const enviar = (e: FormEvent) => {
    e.preventDefault()
    guardar.mutate(c, { onSuccess: onClose })
  }
  return (
    <Dialog open onOpenChange={(o) => !o && onClose()}>
      <DialogContent className="sm:max-w-md">
        <form onSubmit={enviar} className="grid gap-4">
          <DialogHeader>
            <DialogTitle>{c.id ? 'Editar categoría' : 'Nueva categoría'}</DialogTitle>
            <DialogDescription>Después la elegís al cargar o editar cada producto.</DialogDescription>
          </DialogHeader>
          <label className="grid gap-1.5">
            <span className="text-xs font-medium text-muted-foreground">Nombre</span>
            <Input required autoFocus maxLength={60} value={c.nombre ?? ''} placeholder="Bisagras"
              onChange={(e) => setC({ ...c, nombre: e.target.value })} />
          </label>
          <label className="grid gap-1.5">
            <span className="text-xs font-medium text-muted-foreground">Descripción (opcional)</span>
            <Textarea maxLength={300} rows={3} value={c.descripcion ?? ''} placeholder="Cazoletas, codos, cierre suave…"
              onChange={(e) => setC({ ...c, descripcion: e.target.value })} />
          </label>
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

function BorrarCategoria({ categoria, onClose }: { categoria: Categoria; onClose: () => void }) {
  const borrar = useBorrarCategoria()
  return (
    <Dialog open onOpenChange={(o) => !o && onClose()}>
      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>¿Borrar “{categoria.nombre}”?</DialogTitle>
          <DialogDescription>
            {categoria.productos === 0
              ? 'No tiene productos.'
              : categoria.productos === 1
                ? 'El producto que tiene no se borra: queda sin categoría.'
                : `Los ${categoria.productos} productos que tiene no se borran: quedan sin categoría.`}
          </DialogDescription>
        </DialogHeader>
        {borrar.isError && <p role="alert" className="text-sm text-destructive">{borrar.error.message}</p>}
        <DialogFooter>
          <Button variant="outline" onClick={onClose}>Cancelar</Button>
          <Button variant="destructive" disabled={borrar.isPending} onClick={() => borrar.mutate(categoria.id, { onSuccess: onClose })}>
            {borrar.isPending && <Loader2 className="animate-spin" />} Borrar
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
