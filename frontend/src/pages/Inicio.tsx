import { Link } from 'wouter'
import { ArrowRight } from 'lucide-react'
import { Card, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { useUsuario } from '@/auth/sesion'
import { visiblesPara } from '@/layout/navegacion'

const descripciones: Record<string, string> = {
  '/cortes': 'Optimizá el corte de placas',
  '/stock': 'Productos y stock por depósito',
  '/ventas': 'Presupuestos, cobros, boletas y retiros',
}

export default function Inicio() {
  const u = useUsuario()
  const accesos = visiblesPara(u.permisos).flatMap((g) => g.items).filter((i) => i.ruta !== '/')
  const hora = new Date().getHours()
  const saludo = hora < 6 ? 'Buenas noches' : hora < 13 ? 'Buen día' : hora < 20 ? 'Buenas tardes' : 'Buenas noches'
  return (
    <div className="space-y-6">
      <div>
        <h2 className="text-2xl font-semibold tracking-tight">{saludo}, {u.nombre.split(' ')[0]}</h2>
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
                  {descripciones[a.ruta] ?? 'Próximamente'}
                </CardDescription>
              </CardHeader>
            </Card>
          </Link>
        ))}
      </div>
    </div>
  )
}
