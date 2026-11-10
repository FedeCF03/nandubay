// Cliente HTTP para la API de Go. La sesión viaja en una cookie HttpOnly.
export class ErrorApi extends Error {
  estado: number
  constructor(estado: number, mensaje: string) {
    super(mensaje)
    this.estado = estado
  }
}

async function pedir<T>(metodo: string, ruta: string, cuerpo?: unknown): Promise<T> {
  const res = await fetch(ruta, {
    method: metodo,
    credentials: 'same-origin',
    // Todo pedido que cambia datos va como JSON (la API rechaza el resto).
    headers: metodo === 'GET' ? undefined : { 'Content-Type': 'application/json' },
    body: cuerpo === undefined ? undefined : JSON.stringify(cuerpo),
  })
  if (res.status === 204) return undefined as T
  const datos = await res.json().catch(() => ({}))
  if (!res.ok) throw new ErrorApi(res.status, datos.error ?? `Error ${res.status}`)
  return datos as T
}

export const getJSON = <T>(ruta: string) => pedir<T>('GET', ruta)
export const postJSON = <T>(ruta: string, cuerpo?: unknown) => pedir<T>('POST', ruta, cuerpo ?? {})
export const putJSON = <T>(ruta: string, cuerpo: unknown) => pedir<T>('PUT', ruta, cuerpo)
export const delJSON = <T>(ruta: string) => pedir<T>('DELETE', ruta, {})
