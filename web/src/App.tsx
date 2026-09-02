import { useTelegramSetup } from './hooks/useTelegram'
import { Collection } from './pages/Collection'

export function App() {
  useTelegramSetup()
  return <Collection />
}
