import { lazy } from 'react'
import { Route, Switch } from 'wouter'
import { AppLayout } from '@/layout/AppLayout'

// Cada pantalla se descarga recién cuando se entra a ella.
const Inicio = lazy(() => import('@/pages/Inicio'))
const Cortes = lazy(() => import('@/pages/Cortes'))
const Proximamente = lazy(() => import('@/pages/Proximamente'))

export default function App() {
  return (
    <AppLayout>
      <Switch>
        <Route path="/" component={Inicio} />
        <Route path="/cortes" component={Cortes} />
        <Route component={Proximamente} />
      </Switch>
    </AppLayout>
  )
}
