import { Application, Browser, Clipboard, Events, System, Window } from '@wailsio/runtime'
import '../../bindings/events'

export const native = {
  isWindows: () => System.IsWindows(),
  isMac: () => System.IsMac(),
  onCommand: (listener: (id: string) => void) => Events.On('fealdhord:command', event => listener(event.data.id)),
  copy: (text: string) => Clipboard.SetText(text),
  paste: () => Clipboard.Text(),
  close: () => Window.Close(),
  minimize: () => Window.Minimise(),
  isMaximized: () => Window.IsMaximised(),
  onMaximizeChange: (listener: (maximized: boolean) => void) => {
    const maximize = Events.On(Events.Types.Windows.WindowMaximise, () => listener(true))
    const restore = Events.On(Events.Types.Windows.WindowRestore, () => listener(false))
    return () => { maximize(); restore() }
  },
  maximize: () => Window.ToggleMaximise(),
  fullscreen: () => Window.ToggleFullscreen(),
  openURL: (url: string) => Browser.OpenURL(url),
  openDevTools: () => Window.OpenDevTools(),
  quit: () => Application.Quit(),
}
