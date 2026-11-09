import { useState, type FormEvent } from 'react'
import { Loader2 } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { useLogin } from './sesion'

export default function Login() {
  const login = useLogin()
  const [email, setEmail] = useState('')
  const [clave, setClave] = useState('')

  const enviar = (e: FormEvent) => {
    e.preventDefault()
    login.mutate({ email, clave })
  }

  return (
    <div className="flex min-h-svh items-center justify-center bg-[radial-gradient(ellipse_at_top,var(--accent),var(--background)_60%)] p-4">
      <div className="w-full max-w-sm space-y-6">
        <div className="flex flex-col items-center gap-2 text-center">
          <div className="flex size-12 items-center justify-center rounded-xl bg-primary text-xl font-bold text-primary-foreground shadow-sm">
            Ñ
          </div>
          <h1 className="text-2xl font-semibold tracking-tight">Ñandubay</h1>
          <p className="text-sm text-muted-foreground">Gestión para carpinterías</p>
        </div>
        <Card>
          <CardHeader>
            <CardTitle>Entrar</CardTitle>
            <CardDescription>Usá el email y la contraseña de tu usuario.</CardDescription>
          </CardHeader>
          <CardContent>
            <form onSubmit={enviar} className="grid gap-4">
              <div className="grid gap-1.5">
                <Label htmlFor="email">Email</Label>
                <Input id="email" type="email" autoComplete="username" required autoFocus
                  value={email} onChange={(e) => setEmail(e.target.value)} />
              </div>
              <div className="grid gap-1.5">
                <Label htmlFor="clave">Contraseña</Label>
                <Input id="clave" type="password" autoComplete="current-password" required
                  value={clave} onChange={(e) => setClave(e.target.value)} />
              </div>
              {login.isError && (
                <p role="alert" className="text-sm text-destructive">{login.error.message}</p>
              )}
              <Button type="submit" disabled={login.isPending} className="w-full">
                {login.isPending && <Loader2 className="animate-spin" />}
                Entrar
              </Button>
            </form>
          </CardContent>
        </Card>
      </div>
    </div>
  )
}
