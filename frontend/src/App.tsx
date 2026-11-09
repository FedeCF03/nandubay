import { Optimizador } from './cortes/Optimizador'

export default function App() {
  return (
    <div className="min-h-screen bg-slate-100 text-slate-900">
      <header className="no-imprimir bg-amber-900 px-4 py-3 text-white">
        <h1 className="text-lg font-semibold">Ñandubay</h1>
      </header>
      <main className="mx-auto max-w-6xl p-4">
        <h2 className="no-imprimir mb-4 text-xl font-semibold">Optimizador de cortes</h2>
        <Optimizador />
      </main>
    </div>
  )
}
