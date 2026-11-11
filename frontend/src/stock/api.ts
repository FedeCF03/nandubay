import { keepPreviousData, useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { delJSON, getJSON, postJSON, putJSON } from '@/api/cliente'

export type TipoProducto = 'placa' | 'canto' | 'herraje' | 'otro'

export interface Producto {
  id: number
  tipo: TipoProducto
  sku: string
  nombre: string
  marca: string
  color: string
  unidad: string
  costo: number
  precio_venta: number
  stock_minimo: number
  largo_mm?: number
  ancho_mm?: number
  espesor_mm?: number
  tiene_veta: boolean
  refilado_mm: number
  canto_ancho_mm?: number
  canto_espesor_mm?: number
  activo: boolean
  categoria_id: number | null
  categoria?: string
}

export interface Categoria {
  id: number
  nombre: string
  descripcion: string
  productos: number
}

export interface ProductoConStock extends Producto {
  por_deposito: Record<string, number>
  total: number
  bajo: boolean
}

export interface Deposito { id: number; nombre: string; sucursal_id: number; sucursal: string }

export interface Movimiento {
  id: number
  producto_id: number
  deposito_id: number
  tipo: 'compra' | 'venta' | 'ajuste' | 'traspaso_entrada' | 'traspaso_salida' | 'devolucion'
  cantidad: number
  costo_unitario?: number
  nota?: string
  deposito: string
  usuario: string
  fecha: string
}

// categoria: id de la categoría, 'sin' para los productos sin categoría, o vacío para todas.
export interface Filtro { tipo: string; buscar: string; categoria?: string }

const CLAVE = ['stock']

export function useProductos(f: Filtro, enabled = true) {
  const q = new URLSearchParams()
  if (f.tipo) q.set('tipo', f.tipo)
  if (f.buscar.trim()) q.set('buscar', f.buscar.trim())
  if (f.categoria) q.set('categoria', f.categoria)
  return useQuery({
    queryKey: [...CLAVE, 'productos', f],
    queryFn: () => getJSON<ProductoConStock[]>(`/api/v1/productos?${q}`),
    placeholderData: keepPreviousData,
    enabled,
  })
}

export function useDepositos() {
  return useQuery({
    queryKey: [...CLAVE, 'depositos'],
    queryFn: () => getJSON<Deposito[]>('/api/v1/depositos'),
    staleTime: 5 * 60 * 1000,
  })
}

export function useMovimientos(productoID: number | null) {
  return useQuery({
    queryKey: [...CLAVE, 'movimientos', productoID],
    queryFn: () => getJSON<Movimiento[]>(`/api/v1/productos/${productoID}/movimientos`),
    enabled: productoID != null,
  })
}

export function useGuardarProducto() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (p: Partial<Producto>) =>
      p.id ? putJSON<Producto>(`/api/v1/productos/${p.id}`, p) : postJSON<Producto>('/api/v1/productos', p),
    onSuccess: () => qc.invalidateQueries({ queryKey: CLAVE }),
  })
}

export function useCategorias() {
  return useQuery({
    queryKey: [...CLAVE, 'categorias'],
    queryFn: () => getJSON<Categoria[]>('/api/v1/categorias'),
    staleTime: 60 * 1000,
  })
}

export function useGuardarCategoria() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (c: Partial<Categoria>) =>
      c.id ? putJSON<Categoria>(`/api/v1/categorias/${c.id}`, c) : postJSON<Categoria>('/api/v1/categorias', c),
    // Renombrar cambia el nombre que muestran los productos.
    onSuccess: () => qc.invalidateQueries({ queryKey: CLAVE }),
  })
}

export function useBorrarCategoria() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (id: number) => delJSON<void>(`/api/v1/categorias/${id}`),
    onSuccess: () => qc.invalidateQueries({ queryKey: CLAVE }),
  })
}

export type TipoOperacion = 'ingresos' | 'ajustes' | 'traspasos'

export interface DatosOperacion {
  producto_id: number
  deposito_id: number
  destino_id?: number
  cantidad: number
  costo_unitario?: number
  nota?: string
}

export function useOperacion() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: ({ tipo, datos }: { tipo: TipoOperacion; datos: DatosOperacion }) =>
      postJSON<void>(`/api/v1/stock/${tipo}`, datos),
    onSuccess: () => qc.invalidateQueries({ queryKey: CLAVE }),
  })
}

// Formatos argentinos
const numero = new Intl.NumberFormat('es-AR', { maximumFractionDigits: 3 })
const pesos = new Intl.NumberFormat('es-AR', { style: 'currency', currency: 'ARS', maximumFractionDigits: 0 })
export const fmtNumero = (n: number) => numero.format(n)
export const fmtPesos = (n: number) => pesos.format(n)

export const unidadCorta: Record<string, string> = { placa: 'pl.', metro: 'm', unidad: 'u.' }

export const nombreTipo: Record<TipoProducto, string> = {
  placa: 'Placa', canto: 'Canto', herraje: 'Herraje', otro: 'Otro',
}

export function detalle(p: Producto): string {
  if (p.tipo === 'placa') return `${p.largo_mm} × ${p.ancho_mm} × ${String(p.espesor_mm).replace('.', ',')} mm${p.tiene_veta ? ' · con veta' : ''}`
  if (p.tipo === 'canto' && p.canto_ancho_mm) return `${p.canto_ancho_mm} mm${p.canto_espesor_mm ? ` × ${String(p.canto_espesor_mm).replace('.', ',')} mm` : ''}`
  return [p.marca, p.color].filter(Boolean).join(' · ')
}

// Retazo es un sobrante de placa guardado en un depósito. El largo va en el
// sentido de la veta, como en las placas.
export interface Retazo {
  id: number
  placa_id: number
  placa: string
  deposito_id: number
  deposito: string
  largo: number
  ancho: number
  nota: string
  origen: string
  creado_en: string
}

export function useRetazos(placaID = 0, depositoID = 0, enabled = true) {
  const q = new URLSearchParams()
  if (placaID) q.set('placa_id', String(placaID))
  if (depositoID) q.set('deposito_id', String(depositoID))
  return useQuery({
    queryKey: [...CLAVE, 'retazos', placaID, depositoID],
    queryFn: () => getJSON<Retazo[]>(`/api/v1/retazos?${q}`),
    placeholderData: keepPreviousData,
    enabled,
  })
}

export function useRetazoAcciones() {
  const qc = useQueryClient()
  const refrescar = () => {
    qc.invalidateQueries({ queryKey: [...CLAVE, 'retazos'] })
    qc.invalidateQueries({ queryKey: ['ventas', 'presupuesto'] })
    qc.invalidateQueries({ queryKey: ['presupuestos', 'calcular'] })
  }
  const crear = useMutation({
    mutationFn: (r: { placa_id: number; deposito_id: number; largo: number; ancho: number; nota: string }) =>
      postJSON<Retazo>('/api/v1/retazos', r),
    onSuccess: refrescar,
  })
  const descartar = useMutation({
    mutationFn: (id: number) => postJSON<void>(`/api/v1/retazos/${id}/descartar`),
    onSuccess: refrescar,
  })
  return { crear, descartar }
}
