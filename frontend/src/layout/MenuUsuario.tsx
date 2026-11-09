import { Check, ChevronsUpDown, LogOut, MapPin } from 'lucide-react'
import { Avatar, AvatarFallback } from '@/components/ui/avatar'
import {
  DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuLabel,
  DropdownMenuSeparator, DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { SidebarMenu, SidebarMenuButton, SidebarMenuItem, useSidebar } from '@/components/ui/sidebar'
import { useElegirSucursal, useLogout, useUsuario } from '@/auth/sesion'

const iniciales = (nombre: string) =>
  nombre.split(/\s+/).filter(Boolean).slice(0, 2).map((p) => p[0]!.toUpperCase()).join('')

export function MenuUsuario() {
  const u = useUsuario()
  const { isMobile } = useSidebar()
  const logout = useLogout()
  const elegir = useElegirSucursal()
  const sucursal = u.sucursales.find((s) => s.id === u.sucursal_id)

  return (
    <SidebarMenu>
      <SidebarMenuItem>
        <DropdownMenu>
          <DropdownMenuTrigger asChild>
            <SidebarMenuButton size="lg" className="data-[state=open]:bg-sidebar-accent">
              <Avatar className="size-8 rounded-lg">
                <AvatarFallback className="rounded-lg bg-sidebar-primary text-sidebar-primary-foreground">
                  {iniciales(u.nombre)}
                </AvatarFallback>
              </Avatar>
              <div className="grid flex-1 text-left text-sm leading-tight">
                <span className="truncate font-medium">{u.nombre}</span>
                <span className="truncate text-xs text-sidebar-foreground/60">{sucursal?.nombre}</span>
              </div>
              <ChevronsUpDown className="ml-auto size-4" />
            </SidebarMenuButton>
          </DropdownMenuTrigger>
          <DropdownMenuContent className="w-60" side={isMobile ? 'top' : 'right'} align="end" sideOffset={6}>
            <DropdownMenuLabel className="font-normal">
              <div className="font-medium">{u.nombre}</div>
              <div className="truncate text-xs text-muted-foreground">{u.email}</div>
            </DropdownMenuLabel>
            {u.sucursales.length > 1 && (
              <>
                <DropdownMenuSeparator />
                <DropdownMenuLabel className="text-xs text-muted-foreground">Sucursal</DropdownMenuLabel>
                {u.sucursales.map((s) => (
                  <DropdownMenuItem key={s.id} disabled={elegir.isPending}
                    onSelect={() => s.id !== u.sucursal_id && elegir.mutate(s.id)}>
                    <MapPin />
                    <span className="flex-1">{s.nombre}</span>
                    {s.id === u.sucursal_id && <Check />}
                  </DropdownMenuItem>
                ))}
              </>
            )}
            <DropdownMenuSeparator />
            <DropdownMenuItem onSelect={() => logout.mutate()}>
              <LogOut /> Cerrar sesión
            </DropdownMenuItem>
          </DropdownMenuContent>
        </DropdownMenu>
      </SidebarMenuItem>
    </SidebarMenu>
  )
}
