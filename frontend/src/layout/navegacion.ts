import {
  BarChart3, Hammer, Home, Receipt, Package, Scissors, Sofa, Truck, Users, type LucideIcon,
} from 'lucide-react'

export interface ItemNav { ruta: string; titulo: string; icono: LucideIcon; permiso?: string }

export const navegacion: { grupo: string; items: ItemNav[] }[] = [
  { grupo: 'General', items: [{ ruta: '/', titulo: 'Inicio', icono: Home }] },
  {
    grupo: 'Ventas',
    items: [
      { ruta: '/ventas', titulo: 'Ventas de mostrador', icono: Receipt, permiso: 'pedidos.ver' },
      { ruta: '/presupuestos', titulo: 'Presupuestos de muebles', icono: Hammer, permiso: 'pedidos.ver' },
    ],
  },
  {
    grupo: 'Taller',
    items: [
      { ruta: '/cortes', titulo: 'Cortes', icono: Scissors, permiso: 'cortes.ver' },
      { ruta: '/muebles', titulo: 'Muebles modelo', icono: Sofa, permiso: 'pedidos.ver' },
    ],
  },
  {
    grupo: 'Depósito',
    items: [
      { ruta: '/stock', titulo: 'Stock', icono: Package, permiso: 'productos.ver' },
      { ruta: '/compras', titulo: 'Compras', icono: Truck, permiso: 'compras.ver' },
    ],
  },
  {
    grupo: 'Administración',
    items: [
      { ruta: '/estadisticas', titulo: 'Estadísticas', icono: BarChart3, permiso: 'estadisticas.ver' },
      { ruta: '/usuarios', titulo: 'Usuarios y permisos', icono: Users, permiso: 'usuarios.ver' },
    ],
  },
]

// activo dice si la ruta actual está dentro de la sección (ej: /ventas/12 → Ventas).
export const activo = (actual: string, ruta: string) =>
  ruta === '/' ? actual === '/' : actual === ruta || actual.startsWith(ruta + '/')

export const tituloDe = (ruta: string) =>
  navegacion.flatMap((g) => g.items).find((i) => activo(ruta, i.ruta))?.titulo ?? 'Ñandubay'

export const visiblesPara = (permisos: string[]) =>
  navegacion
    .map((g) => ({ ...g, items: g.items.filter((i) => !i.permiso || permisos.includes(i.permiso)) }))
    .filter((g) => g.items.length > 0)
