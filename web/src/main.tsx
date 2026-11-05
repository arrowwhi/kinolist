import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import { App } from './app/App'
import { ErrorBoundary } from './app/ErrorBoundary'
import '@fontsource/inter/400.css'
import '@fontsource/inter/500.css'
import '@fontsource/inter/600.css'
import '@fontsource/inter/700.css'
import './shared/styles/tokens.css'
import './shared/styles/global.css'
import './widgets/app-shell/shell.css'
import './shared/ui/controls.css'
import './features/collection/collection.css'
import './entities/movie/movie.css'
import './pages/genres/genres.css'
import './pages/filters/filters.css'
import './features/rating/rating.css'
import './features/add-movie/add-movie.css'
import './shared/ui/feedback.css'

const root = document.getElementById('root')
if (!root) throw new Error('root element is missing')
createRoot(root).render(
  <StrictMode>
    <ErrorBoundary>
      <App />
    </ErrorBoundary>
  </StrictMode>,
)
