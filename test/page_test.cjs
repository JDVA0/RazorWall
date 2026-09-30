// Prueba funcional del JavaScript de docs/index.html.
//
// El motor WebAssembly es real (docs/razorwall.wasm). Solo el DOM se
// sustituye por un stub minimo, porque en Node no hay navegador. Asi se
// comprueba el cableado de la pagina de verdad: que el deslizador, el
// raton y el teclado lleguen a razorwall.render con lo esperado.

const fs = require("fs");
const path = require("path");
const vm = require("vm");

const ROOT = path.join(__dirname, "..");
const HTML = fs.readFileSync(path.join(ROOT, "docs", "index.html"), "utf8");

const scripts = [...HTML.matchAll(/<script(?![^>]*\bsrc=)[^>]*>([\s\S]*?)<\/script>/g)];
if (!scripts.length) throw new Error("index.html no tiene script inline");
const code = scripts[scripts.length - 1][1];

let fallos = 0;
function check(nombre, cond, extra) {
  if (cond) console.log("  OK    " + nombre);
  else { console.log("  FALLO " + nombre + (extra ? "  -> " + extra : "")); fallos++; }
}

let ctxCalls = 0;
let ctx2d = {
  putImageData(img, x, y) { ctxCalls++; ctx2d._last = { x, y, w: img.width, h: img.height }; },
  drawImage() { ctxCalls++; },
};

function makeEl(tag) {
  const handlers = {};
  const cls = new Set();
  const el = {
    tagName: (tag || "div").toUpperCase(),
    children: [], className: "", _cls: cls,
    _text: "", innerHTML: "",
    value: "", checked: false,
    width: 0, height: 0, devicePixelRatio: 2, style: {},
    classList: {
      add: (c) => cls.add(c),
      remove: (c) => cls.delete(c),
      contains: (c) => cls.has(c),
    },
    getContext: () => ctx2d,
    toBlob(cb) { cb(new BlobStub()); },
    addEventListener(t, f) { (handlers[t] = handlers[t] || []).push(f); },
    appendChild(c) { this.children.push(c); return c; },
    remove() {},
    click() { (handlers.click || []).forEach((f) => f.call(el)); },
    contains(n) { return n === el || this.children.indexOf(n) >= 0; },
    setPointerCapture() {},
    releasePointerCapture() {},
    fire(t, ev) { (handlers[t] || []).forEach((f) => f.call(el, ev || {})); },
    get textContent() { return this._text; },
    set textContent(v) { this._text = String(v); },
  };
  return el;
}

const wasmBytes = fs.readFileSync(path.join(ROOT, "docs", "razorwall.wasm"));

const canvas = makeEl("canvas");
const slider = makeEl("input");
slider.value = "320";
const hint = makeEl("div");
const bar = makeEl("div");
const s2 = makeEl("input");
const s3 = makeEl("input");
slider.value = "320";
s2.value = "2"; s3.value = "100";
const fatal = makeEl("div");
const fatalT = makeEl("h1");
const fatalP = makeEl("p");
class BlobStub {
  constructor(p) { this.size = p[0]?.length || 0; }
}
const byId = {
  stage: makeEl("div"), w: canvas, bar: bar, hint: hint, s1: slider, s2: s2, s3: s3,
  fatal: fatal, "fatal-t": fatalT, "fatal-p": fatalP,
};

const docHandlers = {};
const winHandlers = {};
const sandbox = {
  console, Math, Date, Object, String, Number, Array, Boolean,
  isNaN, parseInt, setTimeout, clearTimeout, setInterval, clearInterval,
  // el motor lo arranca la propia prueba; el fetch del script se deja
  // fallar para no meter un segundo runtime, y asi se cubre el aviso
  location: { protocol: "http:" },
  fetch: () => Promise.reject(new Error("sin red: el motor lo arranca la prueba")),
  WebAssembly, Uint8Array, Blob: class { constructor(p) { this.size = p[0]?.length || 0; } },
  requestAnimationFrame: (f) => setTimeout(f, 0),
  URL: { createObjectURL: () => "blob:test", revokeObjectURL() {} },
  Image: class { set src(v) { this._s = v; setTimeout(() => this.onload && this.onload(), 0); } },
  // ImageData solo necesita llevar los datos al putImageData del stub
  ImageData: class {
    constructor(data, w, h) { this.data = data; this.width = w; this.height = h; }
  },
  innerWidth: 1440, innerHeight: 900, devicePixelRatio: 2,
  addEventListener: (t, f) => { (winHandlers[t] = winHandlers[t] || []).push(f); },
  document: {
    getElementById: (id) => byId[id] || null,
    createElement: (t) => makeEl(t),
    body: { appendChild() {} },
    addEventListener: (t, f) => { (docHandlers[t] = docHandlers[t] || []).push(f); },
  },
};
sandbox.window = sandbox;
sandbox.globalThis = sandbox;
sandbox.crypto = require("crypto").webcrypto;
sandbox.TextEncoder = TextEncoder;
sandbox.TextDecoder = TextDecoder;
sandbox.performance = performance;

vm.createContext(sandbox);

// wasm_exec.js se evalua dentro del contexto para que "this"/globalThis
// apunten al sandbox y no al global de Node.
vm.runInContext(fs.readFileSync(path.join(ROOT, "docs", "wasm_exec.js"), "utf8"),
  sandbox, { filename: "wasm_exec.js" });

const go = new sandbox.Go();
const fire = (type, ev) => (docHandlers[type] || []).forEach((f) => f(ev || {}));
const fireWin = (type, ev) => (winHandlers[type] || []).forEach((f) => f(ev || {}));
const sleep = (ms) => new Promise((r) => setTimeout(r, ms));

(async () => {
  const { instance } = await WebAssembly.instantiate(wasmBytes, go.importObject);
  go.run(instance).catch(() => {});
  await sleep(150);
  check("window.razorwall registrado", typeof sandbox.razorwall === "object");

  const llamadas = [];
  sandbox._realRender = sandbox.razorwall.render;
  sandbox.razorwall.render = (o) => { llamadas.push(o); return new Uint8Array(8); };

  // El camino progresivo se comprueba aparte, en las pruebas de Go y del
  // wasm. Aqui solo hace falta que la pagina lo invoque bien, asi que se
  // sustituye por una version rapida que registra las llamadas.
  sandbox._bands = [];
  sandbox.razorwall.begin = (o) => {
    llamadas.push(o);
    sandbox._bands = [];          // solo interesan las franjas del ultimo render
    return { gridH: 21, width: 320, height: 180, theme: o.theme || "island" };
  };
  let _rows = 0;
  sandbox.razorwall.step = (n) => { _rows = Math.min(21, _rows + n); return _rows; };
  sandbox.razorwall.band = (y0, y1) => {
    sandbox._bands.push([y0, y1]);
    const b = new Uint8Array(320 * (y1 - y0) * 4);
    b.fill(200);
    return b;
  };

  vm.runInContext(code, sandbox, { filename: "index.html<script>" });
  // arranque: el script llama fit() y prepara el fetch
  sandbox.onRazorwallReady();
  await sleep(60);

  check("el lienzo se dimensiona", canvas.width > 0 && canvas.height > 0,
    canvas.width + "x" + canvas.height);
  check("primer render lanzado", llamadas.length >= 1, "llamadas: " + llamadas.length);

  const ini = llamadas[llamadas.length - 1];
  check("el fondo de hoy usa una semilla con forma de fecha",
    String(ini.seed).length === 8, "seed: " + ini.seed);
  check("la pista de uso aparece", hint._cls.has("show"));

  // --- deslizador ---
  const antes = llamadas.length;
  slider.value = "200";
  slider.fire("input");
  await sleep(40);
  check("el deslizador cambia la densidad",
    llamadas.length > antes && llamadas[llamadas.length - 1].pixels === 200,
    String(llamadas[llamadas.length - 1].pixels));

  // --- deslizador de niveles ---
  const bN = llamadas.length;
  s2.value = "4";            // indice 4 -> 4 tonos
  s2.fire("input");
  await sleep(40);
  check("el deslizador de niveles cambia los tonos",
    llamadas.length > bN && llamadas[llamadas.length - 1].levels === 4,
    String(llamadas[llamadas.length - 1].levels));

  // --- zoom: solo transform, no vuelve a renderizar ---
  const antesZoom = llamadas.length;
  s3.value = "250";
  s3.fire("input");
  check("el zoom no vuelve a renderizar", llamadas.length === antesZoom,
    "llamadas: " + (llamadas.length - antesZoom));
  check("el zoom aplica scale en el lienzo",
    /scale\(2\.5\)/.test(canvas.style.transform || ""),
    canvas.style.transform);

  // --- clic (pulsar y soltar sin mover) genera uno nuevo ---
  const stage = byId.stage;
  const ev = (x, y) => ({ pointerId: 1, clientX: x, clientY: y, preventDefault() {} });
  const b = llamadas.length;
  stage.fire("pointerdown", ev(600, 400));
  stage.fire("pointerup", ev(600, 400));
  await sleep(40);
  const tras = llamadas[llamadas.length - 1];
  check("el clic pide un fondo nuevo", llamadas.length > b);
  check("el nuevo fondo cambia la semilla", tras.seed !== ini.seed,
    ini.seed + " -> " + tras.seed);
  check("el nuevo fondo tira el tema o los niveles",
    tras.theme !== ini.theme || tras.levels !== ini.levels);
  check("el tema nuevo existe", sandbox.razorwall.themes.indexOf(tras.theme) >= 0,
    tras.theme);
  check("los niveles son validos", [64, 32, 16, 8, 4].indexOf(tras.levels) >= 0,
    String(tras.levels));

  // --- espacio ---
  const b2 = llamadas.length;
  fire("keydown", { key: " ", preventDefault() {} });
  await sleep(40);
  check("el espacio tambien genera", llamadas.length > b2);

  // --- arrastrar mueve el fondo y NO genera otro ---
  s3.value = "300"; s3.fire("input");
  const bArr = llamadas.length;
  const antesX = stage.__x;
  stage.fire("pointerdown", ev(600, 400));
  stage.fire("pointermove", ev(700, 470));
  stage.fire("pointerup", ev(700, 470));
  await sleep(40);
  check("arrastrar no genera un fondo nuevo", llamadas.length === bArr,
    "llamadas: " + (llamadas.length - bArr));
  check("arrastrar mueve la imagen",
    /translate\([\d.]+px,[\d.]+px\)/.test(canvas.style.transform || ""),
    canvas.style.transform);

  // --- Mayus conservando tema ---
  const b3 = llamadas.length;
  const previo = llamadas[llamadas.length - 1];
  fire("keydown", { key: "Shift", preventDefault() {} });
  await sleep(40);
  const conShift = llamadas[llamadas.length - 1];
  check("Mayus solo cambia la semilla",
    llamadas.length > b3 && conShift.seed !== previo.seed &&
    conShift.theme === previo.theme && conShift.levels === previo.levels);

  // --- teclas de tema ---
  fire("keydown", { key: "3", preventDefault() {} });
  await sleep(40);
  check("la tecla 3 elige desert",
    llamadas[llamadas.length - 1].theme === "desert",
    llamadas[llamadas.length - 1].theme);

  // --- espejo e interpolado ---
  fire("keydown", { key: "m", preventDefault() {} });
  await sleep(40);
  check("la tecla m activa el espejo",
    llamadas[llamadas.length - 1].mirror === true);
  fire("keydown", { key: "i", preventDefault() {} });
  await sleep(40);
  check("la tecla i activa la interpolacion",
    llamadas[llamadas.length - 1].smooth === true);

  // --- la pista desaparece al interactuar ---
  check("la pista se retira al usar la pagina", hint._cls.has("hide"));

  // --- se pinta en el canvas ---
  check("la imagen se pinta por franjas", ctxCalls > 1, "franjas: " + ctxCalls);
  check("las franjas cubren toda la altura",
    sandbox._bands.length > 0 &&
    sandbox._bands[0][0] === 0 &&
    sandbox._bands[sandbox._bands.length - 1][1] === 180,
    JSON.stringify(sandbox._bands.slice(0, 2)));
  check("las franjas van en orden y sin huecos",
    sandbox._bands.every((b, i) => b[0] === (i === 0 ? 0 : sandbox._bands[i - 1][1])),
    JSON.stringify(sandbox._bands.slice(0, 3)));

  // --- redimensionado ---
  const wAntes = canvas.width;
  sandbox.innerWidth = 800; sandbox.innerHeight = 1400;
  fireWin("resize");
  await sleep(260);
  check("al redimensionar se reajusta el lienzo", canvas.width !== wAntes,
    wAntes + " -> " + canvas.width);

  // --- el motor real sigue en pie ---
  const real = sandbox._realRender({ seed: 20260930, theme: "island", levels: 16, pixels: 320, width: 320, height: 180 });
  check("el motor real responde", real && real.length > 0, real ? real.length + " bytes" : "nada");

  console.log(fallos === 0 ? "\nTODO CORRECTO" : `\n${fallos} FALLOS`);
  process.exit(fallos === 0 ? 0 : 1);
})();