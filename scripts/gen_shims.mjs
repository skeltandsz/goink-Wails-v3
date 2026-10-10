// 从 v3 官方生成的绑定（.js + JSDoc）生成"宽松、但保留具体模型/数组/泛型"的兼容层：
//   - App.ts：保留全部具名方法；模型类型解析为命名空间下的具体类型（如 chapter.Chapter、
//     chapter.Chapter[]）；返回去顶层 null；模型字段：Go 指针字段可选、值字段必填；
//     []byte（v3 base64 string）做 base64 转换。
//   - models.ts：按 v2 包命名空间组织，含泛型接口（apperr.Result<T>、storage.PageResult<T>）。
// 用法：node scripts/gen_shims.mjs
import fs from "node:fs";
import path from "node:path";

const root = path.resolve("frontend/src/lib/bindings/github.com/sigpanic/goink");
const outDir = path.resolve("frontend/src/lib/wailsjs");

const nsFiles = {
  app: "app/models.js",
  apperr: "internal/apperr/models.js",
  chapter: "internal/chapter/models.js",
  character: "internal/character/models.js",
  config: "internal/config/models.js",
  deletion: "internal/deletion/models.js",
  git: "internal/git/models.js",
  imp: "internal/import/models.js",
  llm: "internal/llm/models.js",
  location: "internal/location/models.js",
  novel: "internal/novel/models.js",
  pattern: "internal/pattern/models.js",
  preference: "internal/preference/models.js",
  reader: "internal/reader/models.js",
  remote: "internal/skill/remote/models.js",
  search: "internal/search/models.js",
  session: "internal/session/models.js",
  setting: "internal/setting/models.js",
  skill: "internal/skill/models.js",
  storage: "internal/storage/models.js",
  storyarc: "internal/storyarc/models.js",
  style: "internal/style/models.js",
  timeline: "internal/timeline/models.js",
  update: "internal/update/models.js",
  volume: "internal/volume/models.js",
  writing: "internal/writing/models.js",
};

const byteParams = { SaveAvatar: [0], SaveCover: [1] };

const BUILTINS = new Set([
  "string", "number", "boolean", "void", "any", "unknown", "null", "undefined",
  "object", "Array", "Record", "Promise", "Map", "Set", "Date", "Uint8Array",
  "Partial", "Readonly", "ArrayBuffer", "Error",
]);

function matchPair(s, i, open, close) {
  if (s[i] !== open) return null;
  let depth = 0;
  for (let j = i; j < s.length; j++) {
    if (s[j] === open) depth++;
    else if (s[j] === close) {
      depth--;
      if (depth === 0) return j;
    }
  }
  return null;
}

function pathToNs(abs) {
  const rel = path.relative(root, abs).replace(/\\/g, "/");
  if (rel === "app/models.js") return "app";
  const m = rel.match(/^internal\/(.+)\/models\.js$/);
  if (!m) return null;
  let sub = m[1];
  if (sub === "import") return "imp";
  if (sub === "skill/remote") return "remote";
  if (sub.includes("/")) sub = sub.split("/").pop();
  return sub;
}

// 解析一个生成文件：别名映射、类、typedef、const。
function parseFile(abs) {
  const src = fs.readFileSync(abs, "utf8");
  const selfNs = pathToNs(abs) || (path.relative(root, abs).startsWith("app/") ? "app" : null);
  const aliases = {};
  for (const m of src.matchAll(/import\s*\*\s*as\s+([\w$]+)\s+from\s+"([^"]+)"/g)) {
    const resolved = path.resolve(path.dirname(abs), m[2]);
    aliases[m[1]] = pathToNs(resolved);
  }
  // 类
  const classes = [];
  const cre = /export\s+class\s+(\w+)/g;
  let m;
  while ((m = cre.exec(src)) !== null) {
    const name = m[1];
    const bodyStart = src.indexOf("{", m.index);
    const bodyEnd = matchPair(src, bodyStart, "{", "}");
    const body = src.slice(bodyStart, bodyEnd);
    // 模板参数（位于类前的 JSDoc 中）
    const before = src.slice(0, m.index);
    const docEnd = before.lastIndexOf("*/");
    const docStart = before.lastIndexOf("/**", docEnd);
    const doc = before.slice(docStart, docEnd);
    const tpl = [];
    for (const t of doc.matchAll(/@template\s+(\w+)/g)) tpl.push(t[1]);
    // 字段
    const fields = [];
    const fre = /this\["(\w+)"\]/g;
    let fm;
    while ((fm = fre.exec(body)) !== null) {
      const before = body.slice(0, fm.index);
      const at = before.lastIndexOf("@type");
      let type = "any";
      if (at !== -1) {
        const open = body.indexOf("{", at);
        type = body.slice(open + 1, matchPair(body, open, "{", "}")).trim();
      }
      fields.push({ name: fm[1], type });
    }
    classes.push({ name, tpl, fields });
  }
  // typedef（平衡花括号）
  const typedefs = [];
  const tstart = /@typedef\s*/g;
  let trm;
  while ((trm = tstart.exec(src)) !== null) {
    const open = src.indexOf("{", trm.index);
    const close = matchPair(src, open, "{", "}");
    if (close === null) continue;
    const inner = src.slice(open + 1, close).trim();
    const after = src.slice(close + 1).match(/\s*(\w+)/);
    if (after) typedefs.push({ name: after[1], type: inner });
    tstart.lastIndex = close + 1;
  }
  const consts = [];
  const cere2 = /export\s+const\s+(\w+)/g;
  let xm;
  while ((xm = cere2.exec(src)) !== null) {
    const pre = src.slice(0, xm.index);
    const de = pre.lastIndexOf("*/");
    const ds = pre.lastIndexOf("/**", de);
    const d = pre.slice(ds, de);
    const isEnum = /@enum/.test(d);
    // 取到本行分号前的初始化表达式
    const semi = src.indexOf(";", xm.index);
    const expr = src.slice(xm.index, semi);
    const reTarget = expr.match(/=\s*([\w$]+\.\w+)\s*$/);
    consts.push({
      name: xm[1],
      isEnum,
      isReexport: !!reTarget,
      target: reTarget ? reTarget[1] : null,
    });
  }
  return { src, selfNs, aliases, classes, typedefs, consts };
}

// 解析（可能带平衡泛参）限定类型引用，返回 {ns, name, end}。
function resolveQualified(s, i, ctx) {
  const head = s.slice(i).match(/^([\w$]+)\.(\w+)/);
  if (!head) return null;
  const alias = head[1];
  const isAlias = Object.prototype.hasOwnProperty.call(ctx.aliases, alias) || alias === "$models";
  if (!isAlias) return null;
  const ns = ctx.aliases[alias] || (alias === "$models" ? "app" : null);
  let end = i + head[0].length;
  const c = matchPair(s, end, "<", ">");
  if (c !== null) end = c + 1;
  return { ns, name: head[2], end, external: ns === null };
}

// 解析一个类型字符串：限定引用 → ns.Name；裸本地类型 → ns.Name；其余保留。
// prefix 非空时，对所有解析出的命名空间引用加前缀（如 Models.app.X）。
function resolveType(raw, ctx, prefix = null) {
  const p = (ns, name) => (prefix ? `${prefix}.${ns}.${name}` : `${ns}.${name}`);
  let s = raw;
  // 限定引用
  let out = "";
  let i = 0;
  while (i < s.length) {
    const q = resolveQualified(s, i, ctx);
    if (q) {
      if (q.external) {
        out += "any";
      } else {
        const innerStart = s.indexOf("<", i);
        if (innerStart !== -1 && innerStart < q.end) {
          const inner = s.slice(innerStart + 1, q.end - 1);
          out += `${p(q.ns, q.name)}<${resolveType(inner, ctx, prefix)}>`;
        } else {
          out += p(q.ns, q.name);
        }
      }
      i = q.end;
    } else {
      out += s[i];
      i++;
    }
  }
  s = out;
  const local = new Set([
    ...ctx.classes.map((c) => c.name),
    ...ctx.typedefs.map((t) => t.name),
  ]);
  const tplSet = new Set();
  for (const c of ctx.classes) for (const t of c.tpl) tplSet.add(t);
  return s.replace(/[A-Za-z_]\w*/g, (word, off, str) => {
    if (str[off - 1] === "." || str[off + word.length] === ".") return word;
    if (tplSet.has(word) || BUILTINS.has(word)) return word;
    if (local.has(word) && ctx.selfNs) return p(ctx.selfNs, word);
    return word;
  });
}

// 去掉顶层联合中的 null/undefined。
function stripNull(t) {
  const parts = t.split("|").map((p) => p.trim()).filter((p) => p && p !== "null" && p !== "undefined");
  return [...new Set(parts)].join(" | ") || "void";
}

// 解析所有 models 文件
const parsed = {};
for (const [ns, rel] of Object.entries(nsFiles)) {
  parsed[ns] = parseFile(path.join(root, rel));
}

// ── models.ts ──
let modelsTs = `/* eslint-disable @typescript-eslint/no-explicit-any */
// 由 scripts/gen_shims.mjs 生成。Wails v3 兼容层：按 v2 包命名空间组织模型。
// Go 指针字段可选（field?: T），值字段必填（field: T），并带索引签名。
`;
for (const ns of Object.keys(nsFiles)) {
  const ctx = parsed[ns];
  modelsTs += `\nexport namespace ${ns} {\n`;
  modelsTs += `  export type Loose = { [k: string]: any };\n`;
  for (const c of ctx.classes) {
    const tpl = c.tpl.length ? `<${c.tpl.join(", ")}>` : "";
    modelsTs += `  export interface ${c.name}${tpl} {\n`;
    const seen = new Set();
    for (const f of c.fields) {
      if (seen.has(f.name)) continue;
      seen.add(f.name);
      const rt = resolveType(f.type, ctx);
      const nullable = /(^|[\s|])(null|undefined)([\s|]|$)/.test(rt);
      const base = rt.split("|").map((x) => x.trim()).filter((x) => x && x !== "null" && x !== "undefined").join(" | ") || "any";
      modelsTs += nullable ? `    ${f.name}?: ${base};\n` : `    ${f.name}: ${rt};\n`;
    }
    modelsTs += `    [k: string]: any;\n  }\n`;
  }
  const emitted = new Set();
  for (const c of ctx.classes) emitted.add(c.name);
  for (const t of ctx.typedefs) {
    if (!emitted.has(t.name)) {
      emitted.add(t.name);
      // 对象类型（{...}）→ Loose；其余（如 string[]）解析保留。
      const tt = t.type.trim().startsWith("{")
        ? `{ [k: string]: any }`
        : resolveType(t.type, ctx);
      modelsTs += `  export type ${t.name} = ${tt};\n`;
    }
  }
  for (const c of ctx.consts) {
    if (emitted.has(c.name)) continue;
    emitted.add(c.name);
    if (c.isEnum) {
      modelsTs += `  export type ${c.name} = string;\n`;
    } else if (c.isReexport && c.target) {
      // 类型再导出 → 解析到真实类型
      modelsTs += `  export type ${c.name} = ${resolveType(c.target, ctx)};\n`;
    } else {
      modelsTs += `  export type ${c.name} = { [k: string]: any };\n`;
    }
  }
  modelsTs += `}\n`;
}
fs.writeFileSync(path.join(outDir, "go/models.ts"), modelsTs);

// ── App.ts ──
const appCtx = parseFile(path.join(root, "app/app.js"));
// app.js 的本地类型即 app 命名空间
appCtx.classes = parsed.app.classes;
appCtx.typedefs = parsed.app.typedefs;

const fns = [];
const fnRe = /(\/\*\*[\s\S]*?\*\/)\s*export\s+function\s+(\w+)\s*\(([^)]*)\)/g;
let mm;
while ((mm = fnRe.exec(appCtx.src)) !== null) {
  const [, doc, name, params] = mm;
  const paramTypes = {};
  for (const p of doc.matchAll(/@param\s+\{([^}]*)\}\s+\[?(\w+)/g)) {
    paramTypes[p[2]] = resolveType(p[1].trim(), appCtx, "Models");
  }
  let ret = "any";
  const r = doc.match(/@returns\s+\{([\s\S]*?)\}\s*(?:\n|$)/);
  if (r) {
    let t = r[1].trim();
    const pm = t.match(/\$CancellablePromise<([\s\S]*)>$/);
    if (pm) t = pm[1].trim();
    ret = stripNull(resolveType(t, appCtx, "Models"));
  }
  const names = params.split(",").map((p) => p.replace(/=.*/, "").trim()).filter(Boolean);
  // []byte 参数对外保持 number[]（v2 用法），由 wrapper 内部转 base64。
  const bi = byteParams[name] || [];
  for (const idx of bi) {
    if (names[idx]) paramTypes[names[idx]] = "number[]";
  }
  fns.push({ name, names, paramTypes, ret });
}

let appTs = `/* eslint-disable @typescript-eslint/no-explicit-any */
// 由 scripts/gen_shims.mjs 生成。Wails v3 兼容层：保留 v2 具名方法，
// 类型解析为具体模型、返回去空，实际调用由 v3 官方绑定（app.js）处理。
import * as $app from "@/lib/bindings/github.com/sigpanic/goink/app/app.js";
import type * as Models from "../models";

function toBase64(data: any): string {
  if (typeof data === "string") return data;
  const bytes = data instanceof Uint8Array ? data : new Uint8Array(data ?? []);
  let bin = "";
  for (let i = 0; i < bytes.length; i++) bin += String.fromCharCode(bytes[i]);
  return btoa(bin);
}

`;
for (const f of fns) {
  const sig = f.names.map((n) => `${n}: ${f.paramTypes[n] || "any"}`).join(", ");
  appTs += `export function ${f.name}(${sig}): Promise<${f.ret}> {\n`;
  const bi = byteParams[f.name] || [];
  const args = f.names.map((n, i) =>
    bi.includes(i) ? `toBase64(${n}) as any` : `(${n} as any)`,
  );
  appTs += `  return $app.${f.name}(${args.join(", ")}) as any;\n}\n`;
}
fs.writeFileSync(path.join(outDir, "go/app/App.ts"), appTs);

console.log(`App.ts: ${fns.length} functions; models.ts: ${Object.keys(nsFiles).length} namespaces`);
