import { useEffect, useState } from 'react'

// useGuardado es un useState que se acuerda del valor en este navegador, así
// las opciones no vuelven al valor por defecto al recargar la página.
export function useGuardado<T>(clave: string, inicial: T | (() => T)) {
  const [valor, setValor] = useState<T>(() => {
    try {
      const guardado = localStorage.getItem(clave)
      if (guardado != null) return JSON.parse(guardado) as T
    } catch {
      // Sin almacenamiento (modo privado) o dato roto: se usa el inicial.
    }
    return typeof inicial === 'function' ? (inicial as () => T)() : inicial
  })
  useEffect(() => {
    try {
      localStorage.setItem(clave, JSON.stringify(valor))
    } catch {
      // Si no se puede guardar, la página sigue andando igual.
    }
  }, [clave, valor])
  return [valor, setValor] as const
}
