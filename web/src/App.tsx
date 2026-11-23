import { Route, Routes } from 'react-router'
import { Layout } from './components/Layout'
import { NotFound } from './routes/NotFound'
import { Status } from './routes/Status'

// Modo declarativo do React Router: as telas não usam loaders/actions, e o data router
// custava 56 KB a mais no bundle (medido na fase 4, PROGRESS.md). As telas da spec 0006
// (Visão geral, Eventos, Detalhe) entram na fase 5; por enquanto a raiz é a prova de vida.
export function App() {
  return (
    <Routes>
      <Route element={<Layout />}>
        <Route index element={<Status />} />
        <Route path="*" element={<NotFound />} />
      </Route>
    </Routes>
  )
}
