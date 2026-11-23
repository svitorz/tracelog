import { Route, Routes } from 'react-router'
import { Layout } from './components/Layout'
import { Events } from './routes/Events'
import { NotFound } from './routes/NotFound'
import { Status } from './routes/Status'

// Modo declarativo do React Router: as telas não usam loaders/actions, e o data router
// custava 56 KB a mais no bundle (medido na fase 4, PROGRESS.md). As telas da spec 0006
// entram na fase 5, uma fatia por vez: Eventos (5.1), Detalhe (5.2), Visão geral (5.3). Até a
// 5.3, a raiz continua sendo a prova de vida da conexão.
export function App() {
  return (
    <Routes>
      <Route element={<Layout />}>
        <Route index element={<Status />} />
        <Route path="events" element={<Events />} />
        <Route path="*" element={<NotFound />} />
      </Route>
    </Routes>
  )
}
