import { Link } from 'wouter'
import { Plus } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { usePermiso } from '@/auth/sesion'
import { ListaVentas } from './Ventas'

export default function Presupuestos() {
  const crea = usePermiso('pedidos.editar')
  return (
    <div className="space-y-5">
      <div className="flex flex-wrap items-end justify-between gap-3">
        <div>
          <h2 className="text-2xl font-semibold tracking-tight">Presupuestos de muebles</h2>
          <p className="text-muted-foreground">Muebles a medida con sus cortes, herrajes y placas calculadas.</p>
        </div>
        {crea && <Button asChild><Link href="/presupuestos/nuevo"><Plus /> Nuevo presupuesto</Link></Button>}
      </div>
      <ListaVentas tipo="muebles" />
    </div>
  )
}
