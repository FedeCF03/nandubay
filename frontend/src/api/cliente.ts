// Cliente HTTP mínimo para la API de Go.
export async function postJSON<T>(ruta: string, cuerpo: unknown): Promise<T> {
  const res = await fetch(ruta, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(cuerpo),
  })
  const datos = await res.json().catch(() => ({}))
  if (!res.ok) throw new Error(datos.error ?? `Error ${res.status}`)
  return datos as T
}
