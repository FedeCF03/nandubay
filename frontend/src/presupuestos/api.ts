import { keepPreviousData, useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { getJSON, postJSON, putJSON } from '@/api/cliente'
import type { Cantos, Resultado } from '@/cortes/tipos'
import type { Venta } from '@/ventas/api'

// Tipos que coinciden con backend/internal/dominio/muebles.
export interface PiezaMueble {
  nombre: string
  placa_id: number
  largo: number
  ancho: number
  cantidad: number
  respeta_veta: boolean
  cantos: Cantos
  canto_id?: number
}

export interface Insumo { producto_id: number; cantidad: number }

export interface Mueble {
  nombre: string
  ancho_mm: number
  alto_mm: number
  profundidad_mm: number
  cantidad: number
  precio_unitario: number
  piezas: PiezaMueble[]
  insumos: Insumo[]
}

export interface PlacaCalculada {
  producto_id: number
  nombre: string
  cantidad: number
  aprovechamiento: number
  costo_unitario: number
  costo: number
  plan: Resultado
}

export interface MaterialCalculado {
  producto_id: number
  nombre: string
  unidad: string
  cantidad: number
  costo_unitario: number
  costo: number
}

export interface Materiales {
  placas: PlacaCalculada[]
  cantos: MaterialCalculado[]
  insumos: MaterialCalculado[]
  costo: number
  costo_por_mueble: number[]
}

export interface PresupuestoMuebles {
  venta: Venta
  muebles: Mueble[]
  materiales: Materiales | null
  error_materiales?: string
}

export interface DatosPresupuesto {
  cliente_id: number
  notas: string
  fecha_entrega: string | null
  muebles: Mueble[]
}

export function usePresupuesto(id: number) {
  return useQuery({
    queryKey: ['ventas', 'presupuesto', id],
    queryFn: () => getJSON<PresupuestoMuebles>(`/api/v1/presupuestos/${id}`),
    enabled: id > 0,
  })
}

export function useGuardarPresupuesto() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: ({ id, datos }: { id?: number; datos: DatosPresupuesto }) =>
      id ? putJSON<PresupuestoMuebles>(`/api/v1/presupuestos/${id}`, datos)
        : postJSON<PresupuestoMuebles>('/api/v1/presupuestos', datos),
    onSuccess: (p) => {
      qc.setQueryData(['ventas', 'presupuesto', p.venta.id], p)
      qc.setQueryData(['ventas', 'venta', p.venta.id], p.venta)
      qc.invalidateQueries({ queryKey: ['ventas', 'lista'] })
    },
  })
}

// useMateriales calcula placas, cantos e insumos de los muebles mientras se
// arma el presupuesto. Sin muebles completos no pide nada.
export function useMateriales(muebles: Mueble[] | null) {
  const clave = muebles ? JSON.stringify(muebles) : ''
  return useQuery({
    queryKey: ['presupuestos', 'calcular', clave],
    queryFn: () => postJSON<Materiales>('/api/v1/presupuestos/calcular', { muebles }),
    enabled: Boolean(muebles && muebles.length),
    placeholderData: keepPreviousData,
    retry: false,
    staleTime: 60_000,
  })
}

export const sinCantos = (): Cantos => ({ l1: false, l2: false, a1: false, a2: false })
export const tieneCanto = (c: Cantos) => c.l1 || c.l2 || c.a1 || c.a2
