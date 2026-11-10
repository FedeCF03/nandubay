import { Suspense, useState, type ReactNode } from 'react'
import { Link, useLocation } from 'wouter'
import { Moon, Sun } from 'lucide-react'
import {
  Sidebar, SidebarContent, SidebarFooter, SidebarGroup, SidebarGroupContent, SidebarGroupLabel,
  SidebarHeader, SidebarInset, SidebarMenu, SidebarMenuButton, SidebarMenuItem, SidebarProvider,
  SidebarTrigger, useSidebar,
} from '@/components/ui/sidebar'
import { Separator } from '@/components/ui/separator'
import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'
import { useUsuario } from '@/auth/sesion'
import { MenuUsuario } from './MenuUsuario'
import { activo, tituloDe, visiblesPara } from './navegacion'
import { aplicarTema, temaInicial } from './tema'

function Marca() {
  const u = useUsuario()
  return (
    <div className="flex items-center gap-2 px-2 py-1.5">
      <div className="flex size-8 items-center justify-center rounded-md bg-sidebar-primary font-bold text-sidebar-primary-foreground">
        Ñ
      </div>
      <div className="min-w-0 leading-tight group-data-[collapsible=icon]:hidden">
        <div className="font-semibold">Ñandubay</div>
        <div className="truncate text-xs text-sidebar-foreground/60">{u.carpinteria}</div>
      </div>
    </div>
  )
}

function Menu() {
  const [pathname] = useLocation()
  const { setOpenMobile } = useSidebar()
  const { permisos } = useUsuario()
  return (
    <SidebarContent>
      {visiblesPara(permisos).map((g) => (
        <SidebarGroup key={g.grupo}>
          <SidebarGroupLabel className="text-sidebar-foreground/50">{g.grupo}</SidebarGroupLabel>
          <SidebarGroupContent>
            <SidebarMenu>
              {g.items.map((it) => (
                <SidebarMenuItem key={it.ruta}>
                  <SidebarMenuButton asChild isActive={activo(pathname, it.ruta)} tooltip={it.titulo}>
                    <Link href={it.ruta} onClick={() => setOpenMobile(false)}>
                      <it.icono />
                      <span>{it.titulo}</span>
                    </Link>
                  </SidebarMenuButton>
                </SidebarMenuItem>
              ))}
            </SidebarMenu>
          </SidebarGroupContent>
        </SidebarGroup>
      ))}
    </SidebarContent>
  )
}

function CargandoPagina() {
  return (
    <div className="space-y-4">
      <Skeleton className="h-32 w-full" />
      <Skeleton className="h-64 w-full" />
    </div>
  )
}

export function AppLayout({ children }: { children: ReactNode }) {
  const [pathname] = useLocation()
  const [tema, setTema] = useState(temaInicial)
  const cambiarTema = () => {
    const t = tema === 'oscuro' ? 'claro' : 'oscuro'
    aplicarTema(t)
    setTema(t)
  }

  return (
    <SidebarProvider>
      <Sidebar collapsible="icon" className="no-imprimir">
        <SidebarHeader><Marca /></SidebarHeader>
        <Menu />
        <SidebarFooter><MenuUsuario /></SidebarFooter>
      </Sidebar>
      <SidebarInset>
        <header className="no-imprimir sticky top-0 z-10 flex h-14 items-center gap-2 border-b bg-background/90 px-4 backdrop-blur">
          <SidebarTrigger className="-ml-1" />
          <Separator orientation="vertical" className="mr-1 data-[orientation=vertical]:h-4" />
          <h1 className="text-sm font-medium">{tituloDe(pathname)}</h1>
          <Button variant="ghost" size="icon" className="ml-auto" onClick={cambiarTema}
            aria-label={tema === 'oscuro' ? 'Usar modo claro' : 'Usar modo oscuro'}>
            {tema === 'oscuro' ? <Sun /> : <Moon />}
          </Button>
        </header>
        <main className="mx-auto w-full max-w-6xl p-4 md:p-6">
          <Suspense fallback={<CargandoPagina />}>
            {children}
          </Suspense>
        </main>
      </SidebarInset>
    </SidebarProvider>
  )
}
