// Wails v3 兼容层：把 v2 runtime 的具名函数映射到 @wailsio/runtime。
// 注意事件回调差异：v2 回调直接收到数据，v3 收到事件对象（数据在 .data）。
import { Application, Browser, Events, Window } from "@wailsio/runtime";

/** 在系统默认浏览器中打开 URL。 */
export function BrowserOpenURL(url: string): Promise<void> {
  return Browser.OpenURL(url);
}

/** 订阅事件，回调直接接收事件数据（保持 v2 用法）。返回取消订阅函数。 */
export function EventsOn(
  eventName: string,
  callback: (...data: any[]) => void,
): () => void {
  return Events.On(eventName, (ev) => callback(ev.data));
}

/** 发射事件。 */
export function EventsEmit(eventName: string, data?: any): Promise<boolean> {
  return Events.Emit(eventName, data);
}

/** 退出应用。 */
export function Quit(): Promise<void> {
  return Application.Quit();
}

/** 最小化窗口。 */
export function WindowMinimise(): Promise<void> {
  return Window.Minimise();
}

/** 最大化 / 还原切换。 */
export function WindowToggleMaximise(): Promise<void> {
  return Window.ToggleMaximise();
}

/** 窗口是否已最大化。 */
export function WindowIsMaximised(): Promise<boolean> {
  return Window.IsMaximised();
}

/** 获取窗口尺寸（保持 v2 返回 {w,h}）。 */
export async function WindowGetSize(): Promise<{ w: number; h: number }> {
  const size = await Window.Size();
  return { w: size.width, h: size.height };
}

/** 获取窗口位置。 */
export function WindowGetPosition(): Promise<{ x: number; y: number }> {
  return Window.Position();
}

/** 设置窗口尺寸。 */
export function WindowSetSize(width: number, height: number): Promise<void> {
  return Window.SetSize(width, height);
}

/** 设置窗口位置。 */
export function WindowSetPosition(x: number, y: number): Promise<void> {
  return Window.SetPosition(x, y);
}
