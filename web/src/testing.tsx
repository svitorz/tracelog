import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen } from '@testing-library/react'
import { MemoryRouter, useLocation, useNavigate } from 'react-router'
import { App } from './App'

// Mostra a URL atual e oferece um "voltar", para os testes verificarem o histórico como
// o navegador faria (a paginação depende do botão Voltar; spec 0006, seção 4.3).
function HistoryProbe() {
  const location = useLocation()
  const navigate = useNavigate()
  return (
    <div hidden>
      <output data-testid="location">{location.pathname + location.search}</output>
      <button type="button" data-testid="history-back" onClick={() => void navigate(-1)} />
    </div>
  )
}

/** Renders the whole app at `path` (relative to /ui) with a fresh cache and no retries. */
export function renderApp(path = '/') {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  return render(
    <QueryClientProvider client={client}>
      <MemoryRouter initialEntries={[path]}>
        <App />
        <HistoryProbe />
      </MemoryRouter>
    </QueryClientProvider>,
  )
}

export const currentLocation = () => screen.getByTestId('location').textContent

export const historyBack = () => screen.getByTestId('history-back').click()
