// 全局界面缩放（CSS zoom 方案）。
//
// 背景：Android WebView 里 CSS px 等于 dp，手机视口只有约 280dp，而整套 UI 是按桌面
// 1400×900 设计的（固定宽度侧边栏/聊天面板等），在手机上整体过大。后端 Go 侧在
// Android 上没有可用的缩放 API（Wails beta.28 的 setZoom 是空操作），因此缩放必须在
// Web 层完成。
//
// 方案：在 html 根元素上用 CSS `zoom: var(--goink-ui-scale, 1)` 做整体缩放。
// 不用改 font-size 的 rem 方案：前端有约 92 处 text-[Npx] 和 155 处任意值 px 硬编码，
// rem 管不住这些，zoom 能同时缩放 rem 和 px。
//
// 自适应公式（基准 700、下限 0.35、上限 1）：scale = clamp(设备视口宽度 / 700, 0.35, 1)。
// 宽度取 screen.width（不随 zoom 变化），不要取实时的 window.innerWidth，否则 resize
// 时会被自己的缩放值带跑，详见 baseViewportWidth() 的注释。
// 桌面（非移动 UA）恒为 1，不改变桌面现状。
// 档位优先级：用户在设置里手动选的百分比（30%–120%）> 自适应值。
//
// 持久化 key：goink_ui_scale，值为 "auto" 或数字百分比（如 "40"），写法参照
// hooks/useLayoutState.ts（localStorage + 去抖）。
//
// 注意：index.html 首屏内联脚本会复制这里的计算逻辑（保守 ES5 写法，避免大→小闪烁），
// 修改公式/常量时务必同步修改那份脚本。

const STORAGE_KEY = "goink_ui_scale";
const CSS_VAR = "--goink-ui-scale";
const AUTO_BASE_WIDTH = 700; // 自适应基准布局宽度（dp）
const AUTO_MIN_SCALE = 0.35; // 自适应下限
const MANUAL_MIN_PERCENT = 30; // 手动范围下限
const MANUAL_MAX_PERCENT = 120; // 手动范围上限

export type UiScaleSource = "auto" | "manual";

export interface UiScaleInfo {
  /** 当前生效的缩放比例（如 0.4） */
  scale: number;
  /** 生效档位来源：手动覆盖优先，否则自适应 */
  source: UiScaleSource;
  /** 自适应计算值（手动模式下也返回，供 UI 显示） */
  autoScale: number;
  /** 手动百分比（30–120），自动模式为 null */
  manualPercent: number | null;
  /** 是否移动视口（按 UA 判定） */
  isMobile: boolean;
}

/**
 * 移动判定：优先用 UA 同步判定。后端 binding GetPlatform() 在 Android 上返回
 * os === "android"，但那是异步调用，不作为首屏判定依据。
 */
export function isMobileViewport(): boolean {
  return /Android|iPhone|iPad|iPod|Mobile/i.test(navigator.userAgent);
}

function clamp(value: number, min: number, max: number): number {
  return Math.min(max, Math.max(min, value));
}

/**
 * 自适应要用"设备真实视口宽度"，不能用实时的 window.innerWidth。
 *
 * 为什么：根元素上的 CSS `zoom` 与浏览器页面缩放是同一套机制，zoom 生效后
 * window.innerWidth 报告的是**缩放后**的 CSS px（zoom 0.4 时 280dp 的屏会报 ~700）。
 * 直接拿它重算会得到 innerWidth↑ → scale↑ → 布局更宽 → innerWidth 再↑ 的正反馈，
 * 一次 resize 事件就能把档位从 0.4 推到 1（自适应彻底失效）。
 *
 * 取值优先级（都不随 zoom 变化）：
 *  1. screen.width：设备屏幕宽度（dp），旋转时会自动换边，且不受页面缩放影响；
 *  2. window.__goinkBaseWidth：index.html 在写入 zoom 之前抓的 innerWidth，
 *     那是本会话第一次、且必定未缩放的真实视口宽度；
 *  3. window.innerWidth：前两者都拿不到的环境（几乎没有）才用，只在首帧生效。
 *
 * 已知代价：分屏/小窗模式下 screen.width 仍是整机宽度，自适应不会跟着变窄，
 * 这种场景请手动指定档位（设置 → 界面缩放 → 手动）。
 */
declare global {
  interface Window {
    __goinkBaseWidth?: number;
  }
}

function baseViewportWidth(): number {
  const screenWidth = typeof window.screen?.width === "number" ? window.screen.width : 0;
  if (screenWidth > 0) return screenWidth;
  const pristine = window.__goinkBaseWidth;
  if (typeof pristine === "number" && pristine > 0) return pristine;
  return window.innerWidth;
}

/** 自适应缩放：桌面恒为 1；移动按设备视口宽度对基准 700dp 折算，限幅 [0.35, 1]。 */
function computeAutoScale(): number {
  if (!isMobileViewport()) return 1;
  return clamp(baseViewportWidth() / AUTO_BASE_WIDTH, AUTO_MIN_SCALE, 1);
}

function normalizePercent(value: number): number {
  if (Number.isNaN(value)) return MANUAL_MIN_PERCENT;
  return Math.round(clamp(value, MANUAL_MIN_PERCENT, MANUAL_MAX_PERCENT));
}

interface StoredPref {
  source: UiScaleSource;
  manualPercent: number | null;
}

/** 解析持久化值："auto" 或数字百分比字符串；非法/缺失回落到 auto。 */
function readStoredPref(): StoredPref {
  try {
    const raw = localStorage.getItem(STORAGE_KEY);
    if (raw !== null && raw !== "auto") {
      const percent = parseFloat(raw);
      if (
        !Number.isNaN(percent) &&
        percent >= MANUAL_MIN_PERCENT &&
        percent <= MANUAL_MAX_PERCENT
      ) {
        return { source: "manual", manualPercent: normalizePercent(percent) };
      }
    }
  } catch {
    /* ignore */
  }
  return { source: "auto", manualPercent: null };
}

// 模块级缓存档位偏好：setUiScalePreference 同步更新它（保证滑杆实时预览），
// localStorage 写入走去抖（参照 useLayoutState 的防抖写法）。
let pref: StoredPref | null = null;

function getPref(): StoredPref {
  if (!pref) pref = readStoredPref();
  return pref;
}

let persistTimer: ReturnType<typeof setTimeout> | null = null;

function persistPref(value: string) {
  if (persistTimer) clearTimeout(persistTimer);
  persistTimer = setTimeout(() => {
    try {
      localStorage.setItem(STORAGE_KEY, value);
    } catch {
      /* ignore */
    }
  }, 300);
}

function computeInfo(): UiScaleInfo {
  const p = getPref();
  const autoScale = computeAutoScale();
  const scale =
    p.source === "manual" && p.manualPercent !== null
      ? p.manualPercent / 100
      : autoScale;
  return {
    scale,
    source: p.source,
    autoScale,
    manualPercent: p.manualPercent,
    isMobile: isMobileViewport(),
  };
}

const listeners = new Set<(info: UiScaleInfo) => void>();
let lastInfo: UiScaleInfo | null = null;

function infoChanged(a: UiScaleInfo | null, b: UiScaleInfo): boolean {
  if (!a) return true;
  return (
    a.scale !== b.scale ||
    a.source !== b.source ||
    a.manualPercent !== b.manualPercent
  );
}

/** 把当前生效缩放写入 documentElement 的 CSS 变量，并通知订阅者（仅当变化时）。 */
export function applyUiScale(): UiScaleInfo {
  const info = computeInfo();
  document.documentElement.style.setProperty(CSS_VAR, String(info.scale));
  if (infoChanged(lastInfo, info)) {
    lastInfo = info;
    listeners.forEach((cb) => cb(info));
  }
  return info;
}

/** 返回当前生效值/来源/是否移动（实时计算，不依赖缓存）。 */
export function getUiScaleInfo(): UiScaleInfo {
  return computeInfo();
}

/**
 * 设置档位偏好并立即生效（实时预览）：传 "auto" 恢复自适应，传 30–120 的百分比做手动覆盖。
 * 持久化写入去抖，内存偏好同步更新。
 */
export function setUiScalePreference(value: "auto" | number) {
  if (value === "auto") {
    pref = { source: "auto", manualPercent: null };
    persistPref("auto");
  } else {
    const percent = normalizePercent(value);
    pref = { source: "manual", manualPercent: percent };
    persistPref(String(percent));
  }
  applyUiScale();
}

/** 订阅生效档位变化（给设置面板显示当前档位用），返回取消订阅函数。 */
export function subscribeUiScale(cb: (info: UiScaleInfo) => void): () => void {
  listeners.add(cb);
  return () => {
    listeners.delete(cb);
  };
}

/**
 * 初始化：首屏应用一次缩放，并挂载 resize/orientationchange 监听（去抖重算）。
 * 返回清理函数，供应用卸载 / 热更新时调用。
 */
export function initUiScale(): () => void {
  applyUiScale();
  let debounceTimer: ReturnType<typeof setTimeout> | null = null;
  const onViewportChange = () => {
    if (debounceTimer) clearTimeout(debounceTimer);
    debounceTimer = setTimeout(() => {
      applyUiScale();
    }, 150);
  };
  window.addEventListener("resize", onViewportChange);
  window.addEventListener("orientationchange", onViewportChange);
  return () => {
    if (debounceTimer) clearTimeout(debounceTimer);
    window.removeEventListener("resize", onViewportChange);
    window.removeEventListener("orientationchange", onViewportChange);
    listeners.clear();
    lastInfo = null;
    pref = null;
  };
}
