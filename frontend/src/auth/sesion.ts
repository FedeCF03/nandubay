import { createContext, useContext } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { ErrorApi, getJSON, postJSON } from '@/api/cliente'

export interface Sucursal { id: number; nombre: string }

export interface Usuario {
  id: number
  nombre: string
  email: string
  carpinteria_id: number
  carpinteria: string
  sucursal_id: number
  sucursales: Sucursal[]
  permisos: string[]
}

export const CLAVE_YO = ['auth', 'yo']

export function useYo() {
  return useQuery({
    queryKey: CLAVE_YO,
    queryFn: () => getJSON<Usuario>('/api/v1/auth/yo'),
    retry: (n, err) => !(err instanceof ErrorApi && err.estado === 401) && n < 2,
    staleTime: 5 * 60 * 1000,
  })
}

export function useLogin() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (d: { email: string; clave: string }) => postJSON<void>('/api/v1/auth/login', d),
    onSuccess: () => qc.invalidateQueries({ queryKey: CLAVE_YO }),
  })
}

export function useLogout() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: () => postJSON<void>('/api/v1/auth/logout'),
    onSuccess: async () => {
      // Borrar los datos de la sesión anterior y volver a preguntar quién es (da 401).
      qc.removeQueries({ predicate: (q) => q.queryKey[0] !== CLAVE_YO[0] })
      await qc.resetQueries({ queryKey: CLAVE_YO })
    },
  })
}

export function useElegirSucursal() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (sucursal_id: number) => postJSON<void>('/api/v1/auth/sucursal', { sucursal_id }),
    // Cambian los permisos y los datos: se recarga todo.
    onSuccess: () => qc.invalidateQueries(),
  })
}

export const SesionContext = createContext<Usuario | null>(null)

export function useUsuario(): Usuario {
  const u = useContext(SesionContext)
  if (!u) throw new Error('useUsuario fuera de una sesión')
  return u
}

export function usePermiso(codigo: string): boolean {
  return useUsuario().permisos.includes(codigo)
}
