// Prueba del binario WebAssembly fuera del navegador:
// compara su salida con la de la CLI para los mismos parametros.
const fs = require("fs");
const path = require("path");
const { execFileSync } = require("child_process");
const zlib = require("zlib");

const DOCS = path.join(__dirname, "..", "docs");
const ROOT = path.join(__dirname, "..");

require(path.join(DOCS, "wasm_exec.js"));

// La CLI acepta el nivel por nombre; la API web por numero de tonos.
const LEVEL_NAME = { 4: "poster", 8: "retro", 16: "clasico", 32: "medio", 64: "suave" };

const CASES = [
  { seed: 20260930, theme: "island", levels: 16, pixels: 320, width: 960, height: 540 },
  { seed: 20260930, theme: "neon", levels: 4, pixels: 200, width: 640, height: 360 },
  { seed: 7, theme: "volcano", levels: 64, pixels: 480, width: 800, height: 450 },
  { seed: 99, theme: "desert", levels: 8, pixels: 320, width: 640, height: 360, mirror: true },
  { seed: 1234, theme: "glacier", levels: 16, pixels: 320, width: 640, height: 360, smooth: true },
  { seed: 5150, theme: "jungle", levels: 32, pixels: 160, width: 1200, height: 1200 },
];

function cliRender(o, outPath) {
  const args = [
    "--seed", String(o.seed),
    "--theme", o.theme,
    "--levels", LEVEL_NAME[o.levels],
    "--pixels", String(o.pixels),
    "--size", `${o.width}x${o.height}`,
    "--out", outPath,
  ];
  if (o.mirror) args.push("--mirror");
  if (o.smooth) args.push("--smooth");
  execFileSync(path.join(ROOT, "razorwall"), args, { stdio: "ignore" });
  return fs.readFileSync(outPath);
}

// --- decodificador de PNG minimo -----------------------------------------
// El encoder de Go escribe RGBA de 8 bits sin entrelazado, que es justo lo
// que hay que saber para deshacerlo. Se evita traer una dependencia solo
// para comparar pixeles.

// eslint-disable-next-line no-unused-vars
function ensamblarRGBA(partes, width, height) {
  const out = Buffer.alloc(width * height * 4);
  for (const p of partes) {
    p.rgba.copy(out, p.y0 * width * 4);
  }
  return out;
}

function decodificarPNG(buf) {
  // firma
  if (buf.readUInt32BE(0) !== 0x89504e47) throw new Error("no es un PNG");

  let pos = 8;
  let w = 0, h = 0, bitDepth = 0, colorType = 0;
  const idat = [];
  while (pos < buf.length) {
    const len = buf.readUInt32BE(pos);
    const type = buf.toString("ascii", pos + 4, pos + 8);
    const data = buf.subarray(pos + 8, pos + 8 + len);
    if (type === "IHDR") {
      w = data.readUInt32BE(0);
      h = data.readUInt32BE(4);
      bitDepth = data[8];
      colorType = data[9];
      if (data[12] !== 0) throw new Error("PNG entrelazado, no soportado");
    } else if (type === "IDAT") {
      idat.push(data);
    } else if (type === "IEND") {
      break;
    }
    pos += 12 + len;
  }
  if (bitDepth !== 8) throw new Error("profundidad " + bitDepth + " no soportada");

  const canales = { 0: 1, 2: 3, 4: 2, 6: 4 }[colorType];
  if (!canales) throw new Error("tipo de color " + colorType + " no soportado");

  const raw = zlib.inflateSync(Buffer.concat(idat));
  const bpp = canales;
  const stride = w * bpp;
  const out = Buffer.alloc(w * h * 4);
  let prev = Buffer.alloc(stride);
  let off = 0;

  for (let y = 0; y < h; y++) {
    const filtro = raw[off++];
    const linea = Buffer.from(raw.subarray(off, off + stride));
    off += stride;

    // deshacer el filtro de cada linea
    for (let i = 0; i < stride; i++) {
      const a = i >= bpp ? linea[i - bpp] : 0;
      const b = prev[i];
      const c = i >= bpp ? prev[i - bpp] : 0;
      switch (filtro) {
        case 0: break;
        case 1: linea[i] = (linea[i] + a) & 0xff; break;
        case 2: linea[i] = (linea[i] + b) & 0xff; break;
        case 3: linea[i] = (linea[i] + ((a + b) >> 1)) & 0xff; break;
        case 4: {
          const p = a + b - c;
          const pa = Math.abs(p - a), pb = Math.abs(p - b), pc = Math.abs(p - c);
          const pr = pa <= pb && pa <= pc ? a : pb <= pc ? b : c;
          linea[i] = (linea[i] + pr) & 0xff;
          break;
        }
        default: throw new Error("filtro " + filtro + " desconocido");
      }
    }

    // pasar a RGBA
    for (let x = 0; x < w; x++) {
      const s = x * bpp, d = (y * w + x) * 4;
      if (canales >= 3) {
        out[d] = linea[s]; out[d + 1] = linea[s + 1]; out[d + 2] = linea[s + 2];
      } else {
        out[d] = out[d + 1] = out[d + 2] = linea[s];
      }
      out[d + 3] = canales === 4 || canales === 2 ? linea[s + 3] : 255;
    }
    prev = linea;
  }
  return out;
}

(async () => {
  const go = new Go();
  const bytes = fs.readFileSync(path.join(DOCS, "razorwall.wasm"));
  const { instance } = await WebAssembly.instantiate(bytes, go.importObject);

  // go.run() no resuelve nunca porque main bloquea con select{}; hay que
  // lanzarlo sin await y esperar a que registre la API.
  go.run(instance).catch(() => {});
  await new Promise((r) => setTimeout(r, 200));

  const rw = globalThis.razorwall;
  if (!rw || typeof rw.render !== "function") {
    console.error("FALLO: window.razorwall no se registro");
    process.exit(1);
  }
  console.log("API registrada");
  console.log("  version:", rw.version);
  console.log("  themes: ", rw.themes.join(", "));
  console.log("  bands:  ", rw.bands.join(", "));

  let fallos = 0;
  for (const c of CASES) {
    const t0 = Date.now();
    const png = Buffer.from(rw.render(c));
    const ms = Date.now() - t0;

    const tmp = path.join("/tmp", "wasm-check.png");
    const viaCLI = cliRender(c, tmp);

    const igual = png.equals(viaCLI);
    if (!igual) fallos++;
    console.log(
      `  ${igual ? "OK  " : "DIF "} ${c.theme}/${c.levels}/${c.pixels}` +
      `${c.mirror ? "/mirror" : ""}${c.smooth ? "/smooth" : ""}` +
      ` -> ${png.length} bytes en ${ms} ms` +
      (igual ? "" : `  (CLI: ${viaCLI.length} bytes)`)
    );
  }

  // determinismo tambien en wasm
  const a = Buffer.from(rw.render({ seed: 555, theme: "jungle", levels: 16, pixels: 320, width: 640, height: 360 }));
  const b = Buffer.from(rw.render({ seed: 555, theme: "jungle", levels: 16, pixels: 320, width: 640, height: 360 }));
  if (!a.equals(b)) { console.error("  DIF  dos renders wasm con la misma semilla"); fallos++; }
  else console.log("  OK   determinismo en wasm");

  // --- el render por bandas tiene que dar la misma imagen que el completo ---
  // Es lo que permite a la pagina ir pintando: si las bandas no coincidieran,
  // el fondo que se ve mientras se genera seria distinto del que se guarda.
  // Se comparan los bytes RGBA crudos de las bandas con los pixeles del PNG
  // completo, asi que hace falta decodificarlo.
  const CASOS_BANDA = [
    { seed: 20260930, theme: "island", levels: 16, pixels: 160, width: 640, height: 360 },
    { seed: 7, theme: "neon", levels: 4, pixels: 120, width: 480, height: 270 },
    { seed: 99, theme: "volcano", levels: 8, pixels: 160, width: 640, height: 360, mirror: true },
    { seed: 555, theme: "glacier", levels: 32, pixels: 160, width: 640, height: 360, smooth: true },
    { seed: 31415, theme: "desert", levels: 64, pixels: 200, width: 800, height: 450 },
  ];

  for (const c of CASOS_BANDA) {
    const info = rw.begin(c);
    if (!info) { console.log(`  FALLO begin ${c.theme}`); fallos++; continue; }

    // el campo se rellena a trozos, como hara la pagina
    const paso = Math.max(1, Math.ceil(info.gridH / 6));
    let filas = 0;
    while (filas < info.gridH) filas = rw.step(paso);

    // y la imagen se pinta en franjas horizontales
    const FRANJAS = 9;
    const alto = Math.ceil(info.height / FRANJAS);
    const partes = [];
    for (let y = 0; y < info.height; y += alto) {
      const y1 = Math.min(info.height, y + alto);
      partes.push({ y0: y, y1, rgba: Buffer.from(rw.band(y, y1)) });
    }
    const ensamblado = ensamblarRGBA(partes, info.width, info.height);

    // referencia: el render de una pasada, decodificado
    const completo = decodificarPNG(Buffer.from(rw.render(c)));

    const igual = ensamblado.equals(completo);
    if (!igual) fallos++;
    console.log(
      `  ${igual ? "OK  " : "DIF "} ${partes.length} bandas  ${c.theme}` +
      `${c.mirror ? "/mirror" : ""}${c.smooth ? "/smooth" : ""}` +
      `  campo por trozos de ${paso} filas`
    );
  }

// theme vacio / invalido no debe reventar el navegador
  const r = rw.render({ seed: 1, theme: "", levels: 16, pixels: 320, width: 320, height: 180 });
  if (!r || !r.length) { console.error("  FALLO theme vacio devolvio nada"); fallos++; }
  else console.log("  OK   theme vacio elige uno al azar");

  console.log(fallos === 0 ? "\nTODO CORRECTO" : `\n${fallos} FALLOS`);
  process.exit(fallos === 0 ? 0 : 1);
})();