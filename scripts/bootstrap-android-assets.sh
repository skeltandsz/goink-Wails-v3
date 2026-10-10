#!/usr/bin/env bash
# 引导 Wails v3 的 Android 构建脚手架（build/android/）。
#
# 背景：goink 是桌面项目，历史上只用过 desktop 构建，仓库里没有 build/android/
# 目录；根 Taskfile.yml 的 include 写了 `android: optional: true`，所以本地跑
# `wails3 task android:*` 只会得到"task not found"，不会有任何提示。
#
# 而 `wails3 update build-assets` 只同步 updatable_build_assets（darwin/linux/windows
# 的 plist、nsis 工具、info.json、desktop 文件），不含移动端脚手架；Gradle 工程与
# build/android/Taskfile.yml 由 `wails3 init` 的项目模板产出。本脚本的做法是：用与
# goink 相同版本的 wails3 CLI 在临时目录 init 一个同名的空项目，只把 build/android
# 拷回来。这样脚手架版本与 CLI/go.mod 严格对齐，且不会污染仓库里已有的桌面资产。
#
# 已存在则直接跳过（幂等）：脚手架一旦被提交入库，后续 CI 与本地都不会重复生成。
#
# 用法：
#   bash scripts/bootstrap-android-assets.sh              # 需要 PATH 上有 wails3
#   WAILS_VERSION=v3.0.0-beta.28 bash scripts/bootstrap-android-assets.sh
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$REPO_ROOT"

ANDROID_TASKFILE="build/android/Taskfile.yml"
if [ -f "$ANDROID_TASKFILE" ]; then
  echo "Android 脚手架已存在：$ANDROID_TASKFILE（跳过引导）"
  exit 0
fi

if ! command -v wails3 >/dev/null 2>&1; then
  echo "错误: PATH 上没有 wails3。安装：" >&2
  echo "  go install github.com/wailsapp/wails/v3/cmd/wails3@\$(awk '\$1==\"github.com/wailsapp/wails/v3\"{print \$2; exit}' go.mod)" >&2
  exit 1
fi

# 版本必须与 go.mod 一致：模板内容（Gradle 版本、NDK 约定、任务名）跟 CLI 版本走，
# 用 @latest 生成再交给另一个版本的 CLI 构建，属于自找的不确定性。
WAILS_VERSION="${WAILS_VERSION:-$(awk '$1=="github.com/wailsapp/wails/v3"{print $2; exit}' go.mod)}"
if [ -z "${WAILS_VERSION}" ]; then
  echo "错误: 无法从 go.mod 解析 github.com/wailsapp/wails/v3 版本，且未设置 WAILS_VERSION" >&2
  exit 1
fi

# 项目名称必须与 goink 一致：模板会把 BinaryName 渲染进 Gradle 工程（决定
# lib<name>.so 的名称与加载方式）。用别的名字拷回来会在运行期报找不到 native 库。
APP_NAME="${APP_NAME:-goink}"

TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

echo "用 wails3@${WAILS_VERSION} 在临时目录生成 Android 脚手架 -> ${TMP}"
# -skipgomodtidy: 只需要 build/ 模板，不必为一次性项目联网 tidy。
# 产品元数据从 build/config.yml 取，保持与桌面构建一致。
CFG_VERSION="$(awk '/^  version:/{gsub(/"/,"",$2); print $2; exit}' build/config.yml)"
CFG_IDENTIFIER="$(awk '/^  productIdentifier:/{gsub(/"/,"",$2); print $2; exit}' build/config.yml)"
CFG_PRODUCT="$(awk '/^  productName:/{gsub(/"/,"",$2); print $2; exit}' build/config.yml)"
CFG_COMPANY="$(awk '/^  companyName:/{gsub(/"/,"",$2); print $2; exit}' build/config.yml)"

go install "github.com/wailsapp/wails/v3/cmd/wails3@${WAILS_VERSION}"

wails3 init \
  -n "$APP_NAME" \
  -d "$TMP/proj" \
  -q \
  -skipgomodtidy \
  ${CFG_PRODUCT:+-productname "$CFG_PRODUCT"} \
  ${CFG_VERSION:+-productversion "$CFG_VERSION"} \
  ${CFG_COMPANY:+-productcompany "$CFG_COMPANY"} \
  ${CFG_IDENTIFIER:+-productidentifier "$CFG_IDENTIFIER"}

# 不要写死 "$TMP/proj/build/android"：wails3 init 会把工程建在 -d 指定目录下的
# 同名子目录里（-d "$TMP/proj" -n goink -> $TMP/proj/goink/build/android），
# 而更早的布局是直接落在 -d 本身。写死一层会把"路径没对上"误报成
# "该版本 CLI 不提供 Android 脚手架"（CI 上那次
# "错误: 模板里没有找到 /tmp/tmp.XXXX/proj/build/android/Taskfile.yml" 即此，
# 同一份日志的临时项目结构里明明有 proj/goink/build/android）。
SRC="$(find "$TMP/proj" -maxdepth 4 -type d -path '*/build/android' 2>/dev/null | sort | head -1)"
if [ -z "$SRC" ] || [ ! -f "$SRC/Taskfile.yml" ]; then
  echo "错误: 临时项目里没有 build/android/Taskfile.yml，说明该版本 CLI 不提供 Android 脚手架" >&2
  echo "查找根目录：$TMP/proj" >&2
  echo "临时项目结构：" >&2
  find "$TMP/proj" -maxdepth 4 -type d | sed 's/^/  /' >&2
  exit 1
fi
echo "脚手架源目录：$SRC"

mkdir -p build
cp -R "$SRC" build/android

# --- 就地改造脚手架（这三处都是"项目自有文件"，生成后应由仓库接管并 commit） ---
GRADLE=build/android/app/build.gradle
if [ -n "${CFG_IDENTIFIER:-}" ] && [ -f "$GRADLE" ]; then
  # 只改 applicationId，不改 namespace：MainActivity 等 Java 源码仍在
  # com.wails.app 包下（Taskfile 的 adb 启动写死了 com.wails.app.MainActivity），
  # 改 namespace 会让 R 类与源码包错位。applicationId 允许与 namespace 不同。
  sed -i.bak -E "s/^([[:space:]]*applicationId[[:space:]]+).*/\1\"${CFG_IDENTIFIER}\"/" "$GRADLE"
  rm -f "$GRADLE.bak"
  echo "applicationId -> ${CFG_IDENTIFIER}"
  grep -nE 'applicationId|namespace' "$GRADLE" | sed 's/^/  /'
fi

TASKFILE_ANDROID=build/android/Taskfile.yml
if [ -f "$TASKFILE_ANDROID" ] && grep -q '\-ldflags="-w -s"' "$TASKFILE_ANDROID"; then
  # 与桌面三平台 Taskfile 保持一致的 EXTRA_LDFLAGS 钩子：上游 BUILD_FLAGS 里的
  # -ldflags 是写死的，CI 需要它追加 -X=.../internal/version.Version=<tag>，
  # 否则应用内显示的版本恒为 "dev"（Task 无法从 CLI 覆盖 task 级 vars）。
  sed -i.bak 's/-ldflags="-w -s"/-ldflags="-w -s{{if .EXTRA_LDFLAGS}} {{.EXTRA_LDFLAGS}}{{end}}"/g' "$TASKFILE_ANDROID"
  rm -f "$TASKFILE_ANDROID.bak"
  echo "已为 android Taskfile 注入 EXTRA_LDFLAGS 钩子"
fi

echo "已生成 $(find build/android -type f | wc -l) 个文件："
find build/android -maxdepth 2 -type f -o -maxdepth 2 -type d | sort | sed 's/^/  /'

cat <<'MSG'

下一步（重要）：
  1. 把 build/android 整体 commit 入库，不要每次靠本脚本现场生成——Gradle 工程里的
     versionCode / 权限清单 / MainActivity 需要长期演进并走 review。
  2. build/android/app/build.gradle 已被改动的部分：
       applicationId  ← 取自 build/config.yml 的 productIdentifier
       versionCode / versionName 仍由 CI 每次写入（见 .github/actions/android-setup）
     namespace 保持 com.wails.app 未动：Java 源码包与 adb 启动组件名都在这个包下。
  3. compileSdk / targetSdk 模板已是 35（Google Play 新上架要求 API 35+），
     minSdk 为 21；如需收窄设备范围改这里。
  4. 本地先验证一次：
       wails3 task android:package          # release APK（无 keystore 时用 debug 签名）
       wails3 task android:bundle:fat       # 双 ABI 的 AAB，Play 上传用
MSG
