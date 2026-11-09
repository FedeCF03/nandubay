import { lazy, Suspense } from 'react'
import { Route, Switch } from 'wouter'
import { Loader2 } from 'lucide-react'
import { ErrorApi } from '@/api/cliente'
import { SesionContext, useYo } from '@/auth/sesion'
import { AppLayout } from '@/layout/AppLayout'

// Cada pantalla se descarga recién cuando se entra a ella.
const Login = lazy(() => import('@/auth/Login'))
const Inicio = lazy(() => import('@/pages/Inicio'))
const Cortes = lazy(() => import('@/pages/Cortes'))
const Proximamente = lazy(() => import('@/pages/Proximamente'))

function Cargando() {
  return (
    <div className="flex min-h-svh items-center justify-center text-muted-foreground">
      <Loader2 className="size-6 animate-spin" />
    </div>
  )
}

export default function App() {
  const yo = useYo()
  if (yo.isPending) return <Cargando />
  if (yo.isError) {
    if (yo.error instanceof ErrorApi && yo.error.estado === 401) {
      return <Suspense fallback={<Cargando />}><Login /></Suspense>
    }
    return (
      <div className="flex min-h-svh flex-col items-center justify-center gap-2 p-4 text-center">
        <p className="font-medium">No se pudo conectar con el servidor.</p>
        <button className="text-sm text-primary underline" onClick={() => yo.refetch()}>Reintentar</button>
      </div>
    )
  }
  return (
    <SesionContext.Provider value={yo.data}>
      <AppLayout>
        <Switch>
          <Route path="/" component={Inicio} />
          <Route path="/cortes" component={Cortes} />
          <Route component={Proximamente} />
        </Switch>
      </AppLayout>
    </SesionContext.Provider>
  )
}
