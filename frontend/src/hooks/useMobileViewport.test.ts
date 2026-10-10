import { act, renderHook } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { drawerWidthFor, useMobileViewport } from "./useMobileViewport";

/**
 * jsdom 的 window.innerWidth/innerHeight 是只读访问器，只能重新定义属性；
 * 且 jsdom 不提供 matchMedia，所以这里走的正是 hook 的兜底分支
 * （按 innerWidth/innerHeight 判定）。真机 WebView 有 matchMedia，走
 * matchMedia.matches —— 两条分支阈值必须一致，因此断言只针对行为。
 */
function setViewport(width: number, height: number) {
  Object.defineProperty(window, "innerWidth", {
    value: width,
    writable: true,
    configurable: true,
  });
  Object.defineProperty(window, "innerHeight", {
    value: height,
    writable: true,
    configurable: true,
  });
}

function resizeTo(width: number, height: number) {
  setViewport(width, height);
  act(() => {
    window.dispatchEvent(new Event("resize"));
  });
}

describe("useMobileViewport", () => {
  afterEach(() => {
    vi.restoreAllMocks();
    setViewport(1024, 768);
  });

  it("桌面尺寸不进入紧凑形态", () => {
    setViewport(1440, 900);
    const { result } = renderHook(() => useMobileViewport());
    expect(result.current.compact).toBe(false);
    expect(result.current.width).toBe(1440);
    expect(result.current.height).toBe(900);
  });

  it("横屏手机（800×360）进入紧凑形态", () => {
    setViewport(800, 360);
    const { result } = renderHook(() => useMobileViewport());
    expect(result.current.compact).toBe(true);
  });

  it("1100×500 这类矮窗口也算紧凑（阈值以高度为主）", () => {
    setViewport(1100, 500);
    const { result } = renderHook(() => useMobileViewport());
    expect(result.current.compact).toBe(true);
  });

  it("1280×720 的小桌面窗口不被误判为移动端", () => {
    // 这正是阈值以 height 为主的理由：只看宽会把这类窗口折成抽屉，
    // 桌面用户拖小窗口就发现界面变了。
    setViewport(1280, 720);
    const { result } = renderHook(() => useMobileViewport());
    expect(result.current.compact).toBe(false);
  });

  it("旋转/改尺寸后通过 resize 事件更新形态", () => {
    setViewport(400, 800); // 窄屏：400 <= 700 → 紧凑
    const { result } = renderHook(() => useMobileViewport());
    expect(result.current.compact).toBe(true);

    resizeTo(1600, 1000);
    expect(result.current.compact).toBe(false);

    resizeTo(812, 375); // 横屏手机
    expect(result.current.compact).toBe(true);
  });

  it("卸载后移除 resize 监听（不泄漏 handler）", () => {
    setViewport(1440, 900);
    const removeSpy = vi.spyOn(window, "removeEventListener");
    const { unmount } = renderHook(() => useMobileViewport());
    unmount();
    expect(removeSpy).toHaveBeenCalledWith("resize", expect.any(Function));
  });
});

describe("drawerWidthFor", () => {
  it("按视口 34% 取宽", () => {
    expect(drawerWidthFor(800)).toBe(272);
  });

  it("宽屏上限 340px", () => {
    expect(drawerWidthFor(1600)).toBe(340);
  });

  it("极窄视口下不超过视口 45%（宁可抽屉窄，也不盖住正文）", () => {
    expect(drawerWidthFor(360)).toBe(162); // 45% 上限压过 180px 可读下限
    expect(drawerWidthFor(320)).toBe(144);
    for (const w of [320, 360, 400, 700, 800, 1600]) {
      expect(drawerWidthFor(w)).toBeLessThanOrEqual(Math.round(w * 0.45));
    }
  });

  it("常规视口保留 180px 可读下限", () => {
    // 500×34%=170 < 180，但 45% 上限是 225 > 180，所以下限生效
    expect(drawerWidthFor(500)).toBe(180);
  });
});
