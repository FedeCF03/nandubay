import { Link } from 'wouter'
import { ArrowRight } from 'lucide-react'
import { Card, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { navegacion } from '@/layout/navegacion'

export default function Inicio() {
  const accesos = navegacion.flatMap((g) => g.items).filter((i) => i.ruta !== '/')
  return (
    <div className="space-y-6">
      <div>
        <h2 className="text-2xl font-semibold tracking-tight">Buen día</h2>
        <p className="text-muted-foreground">¿Qué hacemos hoy en el taller?</p>
      </div>
      <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
        {accesos.map((a) => (
          <Link key={a.ruta} href={a.ruta} className="group">
            <Card className="h-full transition-colors group-hover:border-primary/50">
              <CardHeader>
                <div className="mb-2 flex size-9 items-center justify-center rounded-lg bg-accent text-accent-foreground">
                  <a.icono className="size-5" />
                </div>
                <CardTitle className="flex items-center justify-between">
                  {a.titulo}
                  <ArrowRight className="size-4 opacity-0 transition-opacity group-hover:opacity-100" />
                </CardTitle>
                <CardDescription>
                  {a.ruta === '/cortes' ? 'Optimizá el corte de placas' : 'Próximamente'}
                </CardDescription>
              </CardHeader>
            </Card>
          </Link>
        ))}
      </div>
    </div>
  )
}
