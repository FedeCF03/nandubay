import {
  BarChart3, ClipboardList, Home, Package, Scissors, Sofa, Truck, Users, type LucideIcon,
} from 'lucide-react'

export interface ItemNav { ruta: string; titulo: string; icono: LucideIcon }

export const navegacion: { grupo: string; items: ItemNav[] }[] = [
  { grupo: 'General', items: [{ ruta: '/', titulo: 'Inicio', icono: Home }] },
  {
    grupo: 'Taller',
    items: [
      { ruta: '/pedidos', titulo: 'Pedidos', icono: ClipboardList },
      { ruta: '/cortes', titulo: 'Cortes', icono: Scissors },
      { ruta: '/muebles', titulo: 'Muebles modelo', icono: Sofa },
    ],
  },
  {
    grupo: 'Depósito',
    items: [
      { ruta: '/stock', titulo: 'Stock', icono: Package },
      { ruta: '/compras', titulo: 'Compras', icono: Truck },
    ],
  },
  {
    grupo: 'Administración',
    items: [
      { ruta: '/estadisticas', titulo: 'Estadísticas', icono: BarChart3 },
      { ruta: '/usuarios', titulo: 'Usuarios y permisos', icono: Users },
    ],
  },
]

export const tituloDe = (ruta: string) =>
  navegacion.flatMap((g) => g.items).find((i) => i.ruta === ruta)?.titulo ?? 'Ñandubay'
