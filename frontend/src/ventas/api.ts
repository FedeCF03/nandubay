import { keepPreviousData, useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { getJSON, postJSON, putJSON } from '@/api/cliente'

export type Estado = 'presupuesto' | 'confirmado' | 'en_corte' | 'en_armado' | 'listo' | 'entregado' | 'cancelado'

export interface Cliente {
  id: number
  nombre: string
  documento: string
  telefono: string
  email: string
  direccion: string
  notas: string
}

export interface Lugar { id: number; nombre: string; direccion: string; telefono: string }

export interface Linea {
  id: number
  tipo: 'producto' | 'mueble'
  producto_id?: number
  descripcion: string
  unidad: string
  ancho_mm?: number
  alto_mm?: number
  profundidad_mm?: number
  cantidad: number
  precio_unitario: number
  retirado: number
}

export interface Pago {
  id: number
  tipo: 'sena' | 'pago' | 'devolucion'
  monto: number
  medio: string
  nota: string
  usuario: string
  fecha: string
}

export interface Retiro {
  id: number
  sucursal: string
  usuario: string
  nota: string
  fecha: string
  items: { linea_id: number; tipo: Linea['tipo']; descripcion: string; cantidad: number }[]
}

export interface Venta {
  id: number
  numero: number
  estado: Estado
  sucursal: Lugar
  deposito_id: number | null
  deposito: string
  retira: Lugar | null
  cliente: Cliente
  vendedor: string
  lineas: Linea[]
  pagos: Pago[]
  retiros: Retiro[]
  historial: { estado: Estado; usuario: string; fecha: string }[]
  notas: string
  fecha_entrega: string | null
  creado_en: string
  confirmado_en: string | null
  total: number
  pagado: number
  saldo: number
  pagada: boolean
  retira_aqui: boolean
  todo_retirado: boolean
  carpinteria: string
  estados_produccion: Estado[]
}

export interface ResumenVenta {
  id: number
  numero: number
  estado: Estado
  cliente: string
  sucursal: string
  retira: string
  total: number
  saldo: number
  fecha: string
}

export interface LineaNueva {
  tipo: Linea['tipo']
  producto_id?: number
  descripcion: string
  ancho_mm?: number
  alto_mm?: number
  profundidad_mm?: number
  cantidad: number
  precio_unitario?: number
}

export interface DatosVenta {
  cliente_id: number
  notas: string
  fecha_entrega: string | null
  lineas: LineaNueva[]
}

const CLAVE = ['ventas']

export function useVentas(estado: string, buscar: string) {
  const q = new URLSearchParams()
  if (estado) q.set('estado', estado)
  if (buscar.trim()) q.set('buscar', buscar.trim())
  return useQuery({
    queryKey: [...CLAVE, 'lista', estado, buscar],
    queryFn: () => getJSON<ResumenVenta[]>(`/api/v1/ventas?${q}`),
    placeholderData: keepPreviousData,
  })
}

export function useVenta(id: number) {
  return useQuery({
    queryKey: [...CLAVE, 'venta', id],
    queryFn: () => getJSON<Venta>(`/api/v1/ventas/${id}`),
    enabled: id > 0,
  })
}

export function useClientes(buscar: string) {
  return useQuery({
    queryKey: ['clientes', buscar],
    queryFn: () => getJSON<Cliente[]>(`/api/v1/clientes?buscar=${encodeURIComponent(buscar.trim())}`),
    placeholderData: keepPreviousData,
  })
}

export function useGuardarCliente() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (c: Partial<Cliente>) =>
      c.id ? putJSON<Cliente>(`/api/v1/clientes/${c.id}`, c) : postJSON<Cliente>('/api/v1/clientes', c),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['clientes'] }),
  })
}

export function useGuardarVenta() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: ({ id, datos }: { id?: number; datos: DatosVenta }) =>
      id ? putJSON<Venta>(`/api/v1/ventas/${id}`, datos) : postJSON<Venta>('/api/v1/ventas', datos),
    onSuccess: (v) => {
      qc.setQueryData([...CLAVE, 'venta', v.id], v)
      qc.invalidateQueries({ queryKey: [...CLAVE, 'lista'] })
    },
  })
}

export type AccionVenta =
  | { accion: 'confirmar'; deposito_id: number }
  | { accion: 'cancelar' }
  | { accion: 'estado'; estado: Estado }
  | { accion: 'pagos'; tipo: Pago['tipo']; monto: number; medio: string; nota: string }
  | { accion: 'retiros'; items: { linea_id: number; tipo: Linea['tipo']; cantidad: number }[]; nota: string }

// useAccionVenta hace una operación sobre la venta y deja la respuesta en caché.
export function useAccionVenta(id: number) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: ({ accion, ...cuerpo }: AccionVenta) => postJSON<Venta>(`/api/v1/ventas/${id}/${accion}`, cuerpo),
    onSuccess: (v, a) => {
      qc.setQueryData([...CLAVE, 'venta', id], v)
      qc.invalidateQueries({ queryKey: [...CLAVE, 'lista'] })
      // Confirmar y cancelar mueven stock.
      if (a.accion === 'confirmar' || a.accion === 'cancelar') qc.invalidateQueries({ queryKey: ['stock'] })
    },
  })
}

export const nombreEstado: Record<Estado, string> = {
  presupuesto: 'Presupuesto',
  confirmado: 'Confirmada',
  en_corte: 'En corte',
  en_armado: 'En armado',
  listo: 'Lista',
  entregado: 'Entregada',
  cancelado: 'Cancelada',
}

export const colorEstado: Record<Estado, string> = {
  presupuesto: 'bg-muted text-muted-foreground',
  confirmado: 'bg-sky-500/15 text-sky-800 dark:text-sky-300',
  en_corte: 'bg-amber-500/15 text-amber-800 dark:text-amber-300',
  en_armado: 'bg-orange-500/15 text-orange-800 dark:text-orange-300',
  listo: 'bg-emerald-500/15 text-emerald-800 dark:text-emerald-300',
  entregado: 'bg-primary/15 text-primary',
  cancelado: 'bg-destructive/10 text-destructive line-through',
}

export const nombrePago: Record<Pago['tipo'], string> = { sena: 'Seña', pago: 'Pago', devolucion: 'Devolución' }

export const mediosPago = ['Efectivo', 'Transferencia', 'Tarjeta de débito', 'Tarjeta de crédito', 'Mercado Pago', 'Cheque']

export const numeroVenta = (n: number) => String(n).padStart(6, '0')

const fecha = new Intl.DateTimeFormat('es-AR', { day: '2-digit', month: '2-digit', year: 'numeric' })
const fechaHora = new Intl.DateTimeFormat('es-AR', { day: '2-digit', month: '2-digit', year: '2-digit', hour: '2-digit', minute: '2-digit' })
export const fmtFecha = (s: string) => fecha.format(new Date(s))
export const fmtFechaHora = (s: string) => fechaHora.format(new Date(s))
// Las fechas sin hora (AAAA-MM-DD) se muestran tal cual, sin pasar por zona horaria.
export const fmtDia = (s: string) => s.split('-').reverse().join('/')

export function medidas(l: { ancho_mm?: number; alto_mm?: number; profundidad_mm?: number }) {
  const m = [l.ancho_mm, l.alto_mm, l.profundidad_mm].filter((x) => x)
  return m.length ? `${m.join(' × ')} mm` : ''
}

// Pendiente de retiro de una línea.
export const pendiente = (l: Linea) => Math.max(0, Math.round((l.cantidad - l.retirado) * 1000) / 1000)
