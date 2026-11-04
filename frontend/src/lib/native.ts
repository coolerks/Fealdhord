import { Application, Clipboard, Events, System, Window } from '@wailsio/runtime'
import '../../bindings/events'

export const native = {
  isMac: () => System.IsMac(),
  onCommand: (listener: (id: string) => void) => Events.On('fealdhord:command', event => listener(event.data.id)),
  copy: (text: string) => Clipboard.SetText(text),
  paste: () => Clipboard.Text(),
  close: () => Window.Close(),
  minimize: () => Window.Minimise(),
  maximize: () => Window.ToggleMaximise(),
  fullscreen: () => Window.ToggleFullscreen(),
  quit: () => Application.Quit(),
}
