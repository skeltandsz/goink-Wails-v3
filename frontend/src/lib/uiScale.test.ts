import { beforeEach, describe, expect, it, vi } from "vitest";

import {
  applyUiScale,
  getUiScaleInfo,
  initUiScale,
  setUiScalePreference,
  subscribeUiScale,
} from "./uiScale";

// 界面缩放的纯逻辑回归测试。
//
// 最要紧的一条是 "auto 档不被自己的缩放值带跑"：根元素 zoom 生效后
// window.innerWidth 报告的是缩放后的 CSS px（280dp 的屏在 zoom 0.4 下会报 ~700），
// 若自适应继续读 innerWidth，就会 innerWidth↑ → scale↑ 正反馈一路推到 1，
// 移动端适配彻底失效。因此 auto 计算必须锚在不随 zoom 变化的量上（screen.width）。

function setUa(ua: string) {
  Object.defineProperty(window.navigator, "userAgent", {
    value: ua,
    configurable: true,
  });
}

function setScreen(width: number) {
  Object.defineProperty(window.screen, "width", {
    value: width,
    configurable: true,
  });
}

function setInnerWidth(width: number) {
  Object.defineProperty(window, "innerWidth", {
    value: width,
    configurable: true,
    writable: true,
  });
}

const MOBILE_UA =
  "Mozilla/5.0 (Linux; Android 14; M2011J185C Build/UKQ1.240415.001; wv) " +
  "AppleWebKit/537.36 (KHTML, like Gecko) Version/4.0 Chrome/126.0.0.0 Mobile Safari/537.36";
const DESKTOP_UA =
  "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Safari/537.36";

beforeEach(() => {
  localStorage.clear();
  document.documentElement.style.removeProperty("--goink-ui-scale");
  delete window.__goinkBaseWidth;
  vi.useRealTimers();
  vi.restoreAllMocks();
  // uiScale 模块持有档位缓存（pref/listeners/lastInfo），init 的清理函数是唯一
  // 的重置入口；每个用例前跑一次 init+dispose，避免用例之间互相污染。
  initUiScale()();
});

describe("自适应档位", () => {
  it("桌面恒为 1，不受视口宽度影响", () => {
    setUa(DESKTOP_UA);
    setScreen(1400);
    setInnerWidth(900);
    expect(applyUiScale().scale).toBe(1);
  });

  it("280dp 手机屏折算为 0.4（基准 700）", () => {
    setUa(MOBILE_UA);
    setScreen(280);
    setInnerWidth(280);
    expect(applyUiScale().scale).toBeCloseTo(0.4, 5);
  });

  it("宽度对基准取比值，落到下限与上限之间", () => {
    setUa(MOBILE_UA);
    setInnerWidth(1000);

    setScreen(150); // 150/700 < 0.35 → 取下限
    expect(applyUiScale().scale).toBeCloseTo(0.35, 5);

    setScreen(1200); // 1200/700 > 1 → 取上限
    expect(applyUiScale().scale).toBe(1);
  });

  it("zoom 生效后 innerWidth 变大也不会把档位推高（防正反馈）", () => {
    setUa(MOBILE_UA);
    setScreen(280);
    setInnerWidth(280);
    const first = applyUiScale().scale;

    // 模拟 zoom 0.4 生效后浏览器报告"布局视口变宽了"
    setInnerWidth(280 / first);
    const second = applyUiScale().scale;
    const third = getUiScaleInfo().scale;

    expect(second).toBeCloseTo(first, 5);
    expect(third).toBeCloseTo(first, 5);
  });

  it("screen.width 不可用时退回首屏抓到的 __goinkBaseWidth", () => {
    setUa(MOBILE_UA);
    setScreen(0);
    window.__goinkBaseWidth = 350;
    setInnerWidth(875); // 已被 zoom 放大过的读数，不该被采用
    expect(applyUiScale().scale).toBeCloseTo(0.5, 5);
  });
});

describe("手动档位", () => {
  beforeEach(() => {
    setUa(MOBILE_UA);
    setScreen(280);
    setInnerWidth(280);
  });

  it("手动百分比优先于自适应，并在去抖后写入 localStorage", async () => {
    setUiScalePreference(60);
    const info = getUiScaleInfo();
    expect(info.source).toBe("manual");
    expect(info.scale).toBe(0.6);
    expect(info.autoScale).toBeCloseTo(0.4, 5); // 自动值仍随行情计算，供 UI 显示

    // 落盘走 300ms 去抖（与 hooks/useLayoutState 的写法一致），等它写完再断言
    await new Promise((resolve) => setTimeout(resolve, 400));
    expect(localStorage.getItem("goink_ui_scale")).toBe("60");
  });

  it("越界与非法输入被收敛", () => {
    setUiScalePreference(200);
    expect(getUiScaleInfo().manualPercent).toBe(120);
    setUiScalePreference(-5);
    expect(getUiScaleInfo().manualPercent).toBe(30);
  });

  it('传 "auto" 回到自适应', () => {
    setUiScalePreference(80);
    setUiScalePreference("auto");
    const info = getUiScaleInfo();
    expect(info.source).toBe("auto");
    expect(info.manualPercent).toBeNull();
    expect(info.scale).toBeCloseTo(0.4, 5);
  });

  it("localStorage 里是非法值时回落 auto，而不是抛错", () => {
    localStorage.setItem("goink_ui_scale", "not-a-number");
    expect(applyUiScale().scale).toBeCloseTo(0.4, 5);

    localStorage.setItem("goink_ui_scale", "999");
    expect(applyUiScale().scale).toBeCloseTo(0.4, 5);
  });
});

describe("订阅", () => {
  it("只在档位变化时通知订阅者，取消订阅后不再收到", () => {
    setUa(MOBILE_UA);
    setScreen(280);
    setInnerWidth(280);

    const cb = vi.fn();
    const unsubscribe = subscribeUiScale(cb);

    applyUiScale();
    applyUiScale(); // 同一档位，不应重复通知
    expect(cb).toHaveBeenCalledTimes(1);

    unsubscribe();
    setUiScalePreference(70);
    expect(cb).toHaveBeenCalledTimes(1);
  });
});
