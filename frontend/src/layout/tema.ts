// Modo claro/oscuro. Se recuerda en el navegador si se puede.
const CLAVE = 'nandubay-tema'

export function temaInicial(): 'claro' | 'oscuro' {
  try {
    const t = localStorage.getItem(CLAVE)
    if (t === 'claro' || t === 'oscuro') return t
  } catch { /* sin almacenamiento */ }
  return window.matchMedia?.('(prefers-color-scheme: dark)').matches ? 'oscuro' : 'claro'
}

export function aplicarTema(t: 'claro' | 'oscuro') {
  document.documentElement.classList.toggle('dark', t === 'oscuro')
  try { localStorage.setItem(CLAVE, t) } catch { /* sin almacenamiento */ }
}
