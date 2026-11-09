// Tipos que coinciden con backend/internal/cortes.
export interface Cantos { l1: boolean; l2: boolean; a1: boolean; a2: boolean }

export interface Pieza {
  id: string
  etiqueta: string
  largo: number
  ancho: number
  cantidad: number
  respeta_veta: boolean
  cantos: Cantos
}

export interface Parametros {
  placa_largo: number
  placa_ancho: number
  tiene_veta: boolean
  kerf: number
  refilado: number
  sobrante_min_largo: number
  sobrante_min_ancho: number
}

export interface Rect { x: number; y: number; largo: number; ancho: number }

export interface Colocacion extends Rect { pieza_id: string; etiqueta: string; rotada: boolean }

export interface PlacaUsada {
  indice: number
  retazo_id?: string
  largo: number
  ancho: number
  piezas: Colocacion[]
  sobrantes: Rect[] | null
  aprovechamiento: number
}

export interface Resultado {
  placas: PlacaUsada[]
  placas_enteras: number
  retazos_usados: number
  no_colocadas: Pieza[]
  aprovechamiento: number
  metros_canto: number
}
