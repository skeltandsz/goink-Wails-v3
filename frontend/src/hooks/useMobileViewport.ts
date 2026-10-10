import { useEffect, useState } from "react";

/**
 * 移动端（含横屏平板/手机）视口判定。
 *
 * 为什么以 height 为主：Android 侧把 Activity 锁成 sensorLandscape，横屏手机的
 * 特征不是"窄"而是"矮"（典型 800×360 CSS px），而桌面窗口极少矮到 520px。
 * 只看 width 会把常见的 1280×720 小笔记本窗口误判成移动端，把三栏收进抽屉，
 * 桌面用户会觉得"窗口缩小一点界面就变了"。width 条件只作为平板分屏等极端情况的
 * 兜底。
 *
 * 两个条件都写在同一个 media query 里，保证"初始渲染"与"之后变化"用同一套判断，
 * 不会出现首屏按桌面渲染、下一帧又切成抽屉的抖动。
 */
const COMPACT_QUERY = "(max-height: 520px), (max-width: 700px)";
const COMPACT_MAX_HEIGHT = 520;
const COMPACT_MAX_WIDTH = 700;

export interface MobileViewport {
  /** 是否进入移动端形态（三栏折叠为抽屉、字号收紧） */
  compact: boolean;
  /** 当前视口宽度（CSS px），用于把侧栏抽屉限制在屏幕的一部分 */
  width: number;
  /** 当前视口高度（CSS px） */
  height: number;
}

function measure(): MobileViewport {
  // SSR / jsdom 无 window.matchMedia：回退到 innerWidth/innerHeight，两者都拿不到
  // 时按桌面形态（compact=false），保证测试环境不因缺少浏览器 API 而崩。
  if (typeof window === "undefined") {
    return { compact: false, width: 1280, height: 800 };
  }
  const width = window.innerWidth || 1280;
  const height = window.innerHeight || 800;
  const mq = window.matchMedia?.(COMPACT_QUERY);
  const compact = mq
    ? mq.matches
    : height <= COMPACT_MAX_HEIGHT || width <= COMPACT_MAX_WIDTH;
  return { compact, width, height };
}

/**
 * 订阅视口形态变化。WebView 在旋转、DPI/字体缩放变化、分屏拖动时都会发 resize，
 * 所以监听 resize 与 matchMedia change 两条路径（后者在不触发 resize 的系统级
 * 偏好变化时兜底）。
 */
export function useMobileViewport(): MobileViewport {
  const [vp, setVp] = useState<MobileViewport>(measure);

  useEffect(() => {
    const sync = () => setVp(measure());

    const mq =
      typeof window !== "undefined"
        ? window.matchMedia?.(COMPACT_QUERY)
        : undefined;
    mq?.addEventListener?.("change", sync);
    window.addEventListener("resize", sync);
    // 首帧之后系统可能已经改变了视觉视口（WebView 完成布局才定尺寸），补一次。
    sync();

    return () => {
      mq?.removeEventListener?.("change", sync);
      window.removeEventListener("resize", sync);
    };
  }, []);

  return vp;
}

/**
 * 抽屉宽度：按视口 34% 取，上限 340px，且任何情况下不超过视口的 45%。
 * 45% 这一条优先于 180px 的可读下限：两者冲突时（视口窄到 400px 以下）如果还
 * 坚持 180px，抽屉会盖掉过半屏幕，正文就没法看了 —— 宁可抽屉窄一点。
 */
export function drawerWidthFor(viewportWidth: number): number {
  const maxByViewport = Math.round(viewportWidth * 0.45);
  const floor = Math.min(180, maxByViewport);
  const cap = Math.min(340, maxByViewport);
  const desired = Math.round(viewportWidth * 0.34);
  return Math.max(floor, Math.min(cap, desired));
}
