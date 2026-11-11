import { lazy, Suspense } from 'react'
import { Route, Switch, useRoute } from 'wouter'
import { Loader2 } from 'lucide-react'
import { ErrorApi } from '@/api/cliente'
import { SesionContext, useYo } from '@/auth/sesion'
import { AppLayout } from '@/layout/AppLayout'

// Cada pantalla se descarga recién cuando se entra a ella.
const Login = lazy(() => import('@/auth/Login'))
const Inicio = lazy(() => import('@/pages/Inicio'))
const Cortes = lazy(() => import('@/pages/Cortes'))
const Stock = lazy(() => import('@/pages/Stock'))
const Ventas = lazy(() => import('@/pages/Ventas'))
const Venta = lazy(() => import('@/pages/Venta'))
const FormVenta = lazy(() => import('@/pages/FormVenta'))
const Presupuestos = lazy(() => import('@/pages/Presupuestos'))
const FormPresupuesto = lazy(() => import('@/pages/FormPresupuesto'))
const MuebleDetalle = lazy(() => import('@/pages/MuebleDetalle'))
const PlanoCortes = lazy(() => import('@/pages/PlanoCortes'))
const Boleta = lazy(() => import('@/pages/Boleta'))
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
  // La boleta se muestra sola, sin menú, para imprimirla en A4.
  const [esBoleta] = useRoute('/ventas/:id/boleta')
  const [esPlano] = useRoute('/presupuestos/:id/cortes')
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
      {esBoleta || esPlano ? <Suspense fallback={<Cargando />}>{esBoleta ? <Boleta /> : <PlanoCortes />}</Suspense> : (
      <AppLayout>
        <Switch>
          <Route path="/" component={Inicio} />
          <Route path="/cortes" component={Cortes} />
          <Route path="/stock" component={Stock} />
          <Route path="/ventas" component={Ventas} />
          <Route path="/ventas/nueva" component={FormVenta} />
          <Route path="/ventas/:id/editar" component={FormVenta} />
          <Route path="/ventas/:id" component={Venta} />
          <Route path="/presupuestos" component={Presupuestos} />
          <Route path="/presupuestos/nuevo" component={FormPresupuesto} />
          <Route path="/presupuestos/:id/editar" component={FormPresupuesto} />
          <Route path="/presupuestos/:id/muebles/:n" component={MuebleDetalle} />
          <Route path="/presupuestos/:id" component={Venta} />
          <Route component={Proximamente} />
        </Switch>
      </AppLayout>
      )}
    </SesionContext.Provider>
  )
}
