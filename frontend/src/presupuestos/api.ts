import { keepPreviousData, useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { delJSON, getJSON, postJSON, putJSON } from '@/api/cliente'
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
  veta_vertical?: boolean
  cantos: Cantos
  canto_id?: number
}

export interface Insumo { producto_id: number; cantidad: number }

export interface Mueble {
  clave?: string
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
  cantidad: number // placas enteras
  retazos: number // retazos usados (no se cobran)
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
  precio_por_mueble?: number[]
  canto_por_mueble?: number[]
}

// RECARGO es por cuánto se multiplica el costo de materiales para el precio
// (igual que muebles.Recargo en el backend).
export const RECARGO = 2.7

export interface PresupuestoMuebles {
  venta: Venta
  muebles: Mueble[]
  materiales: Materiales | null
  error_materiales?: string
  fotos: FotoMueble[]
}

export interface FotoMueble { id: number; mueble_clave: string; creado_en: string }

export const urlFoto = (pedidoId: number, fotoId: number) => `/api/v1/presupuestos/${pedidoId}/fotos/${fotoId}`

// achicarFoto pasa la foto a JPEG de hasta 1600 px de lado, así una foto del
// celular (varios MB) se sube rápido y ocupa poco. Devuelve el base64.
export async function achicarFoto(archivo: File, lado = 1600): Promise<string> {
  const bmp = await createImageBitmap(archivo)
  const escala = Math.min(1, lado / Math.max(bmp.width, bmp.height))
  const canvas = document.createElement('canvas')
  canvas.width = Math.round(bmp.width * escala)
  canvas.height = Math.round(bmp.height * escala)
  canvas.getContext('2d')!.drawImage(bmp, 0, 0, canvas.width, canvas.height)
  bmp.close()
  const url = canvas.toDataURL('image/jpeg', 0.82)
  return url.slice(url.indexOf(',') + 1)
}

export function useFotos(pedidoId: number) {
  const qc = useQueryClient()
  const refrescar = () => qc.invalidateQueries({ queryKey: ['ventas', 'presupuesto', pedidoId] })
  const subir = useMutation({
    mutationFn: async ({ clave, archivos }: { clave: string; archivos: File[] }) => {
      for (const a of archivos) {
        const datos = await achicarFoto(a).catch(() => {
          throw new Error(`No se pudo leer ${a.name}: tiene que ser una imagen.`)
        })
        await postJSON<FotoMueble>(`/api/v1/presupuestos/${pedidoId}/fotos`, { mueble_clave: clave, datos })
      }
    },
    onSettled: refrescar,
  })
  const borrar = useMutation({
    mutationFn: (fotoId: number) => delJSON<void>(urlFoto(pedidoId, fotoId)),
    onSettled: refrescar,
  })
  return { subir, borrar }
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

// cuantasPlacas dice "2 placas + 1 retazo".
export function cuantasPlacas(placas: number, retazos = 0) {
  const t = []
  if (placas || !retazos) t.push(`${placas} ${placas === 1 ? 'placa' : 'placas'}`)
  if (retazos) t.push(`${retazos} ${retazos === 1 ? 'retazo' : 'retazos'}`)
  return t.join(' + ')
}
