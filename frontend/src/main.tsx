import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import "./index.css";
import "./i18n";
import App from "./App.tsx";
import { initUiScale } from "./lib/uiScale";

// 全局界面缩放初始化（移动端 WebView 适配，详见 src/lib/uiScale.ts），
// 热更新/卸载时清理监听，避免重复挂载。
const disposeUiScale = initUiScale();
if (import.meta.hot) {
  import.meta.hot.dispose(() => disposeUiScale());
}

createRoot(document.getElementById("root")!).render(
  <StrictMode>
    <App />
  </StrictMode>,
);
