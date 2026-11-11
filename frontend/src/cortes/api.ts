import { keepPreviousData, useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { getJSON, postJSON } from '@/api/cliente'
import type { Parametros, Pieza, Resultado } from './tipos'

export type EstadoCorte = 'pendiente' | 'cortado' | 'anulado'

// CorteOptimizado coincide con aplicacion.CorteOptimizado.
export interface CorteOptimizado {
  id: number
  nombre: string
  pedido_id: number | null
  pedido_numero: number | null
  cliente: string
  placa_id: number
  placa: string
  deposito_id: number
  deposito: string
  estado: EstadoCorte
  parametros: Parametros
  piezas: Pieza[] | null
  resultado: Resultado
  metros_canto: number
  creado_en: string
  creado_por: string
  cortado_en: string | null
  cortado_por: string
}

export interface CorteNuevo {
  nombre: string
  placa_id: number
  deposito_id: number
  kerf: number
  piezas: Pieza[]
}

export const nombreEstadoCorte: Record<EstadoCorte, string> = { pendiente: 'Pendiente', cortado: 'Cortado', anulado: 'Anulado' }

export const colorEstadoCorte: Record<EstadoCorte, string> = {
  pendiente: 'bg-amber-100 text-amber-900 dark:bg-amber-900/40 dark:text-amber-100',
  cortado: 'bg-emerald-100 text-emerald-900 dark:bg-emerald-900/40 dark:text-emerald-100',
  anulado: 'bg-muted text-muted-foreground',
}

// tituloCorte: el nombre que se le puso, o la venta de la que sale.
export const tituloCorte = (c: CorteOptimizado) =>
  c.nombre || (c.pedido_numero ? `Venta N° ${String(c.pedido_numero).padStart(6, '0')}${c.cliente ? ` · ${c.cliente}` : ''}` : `Corte #${c.id}`)

export function useCortes(estado: string) {
  return useQuery({
    queryKey: ['cortes', 'lista', estado],
    queryFn: () => getJSON<CorteOptimizado[]>(`/api/v1/cortes${estado ? `?estado=${estado}` : ''}`),
    placeholderData: keepPreviousData,
  })
}

export function useCorte(id: number) {
  return useQuery({
    queryKey: ['cortes', 'corte', id],
    queryFn: () => getJSON<CorteOptimizado>(`/api/v1/cortes/${id}`),
    enabled: id > 0,
  })
}

export function useCrearCorte() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (n: CorteNuevo) => postJSON<CorteOptimizado>('/api/v1/cortes', n),
    onSuccess: (c) => {
      qc.setQueryData(['cortes', 'corte', c.id], c)
      qc.invalidateQueries({ queryKey: ['cortes', 'lista'] })
    },
  })
}

// useAccionCorte confirma que se cortó, recalcula o anula. Confirmar cambia
// el stock y los retazos, así que se refresca todo eso.
export function useAccionCorte(id: number) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (accion: 'confirmar' | 'recalcular' | 'anular') => postJSON<CorteOptimizado>(`/api/v1/cortes/${id}/${accion}`, {}),
    onSuccess: (c) => {
      qc.setQueryData(['cortes', 'corte', id], c)
      qc.invalidateQueries({ queryKey: ['cortes', 'lista'] })
      qc.invalidateQueries({ queryKey: ['stock'] })
    },
  })
}
