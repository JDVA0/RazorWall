// Prueba del binario WebAssembly fuera del navegador:
// compara su salida con la de la CLI para los mismos parametros.
const fs = require("fs");
const path = require("path");
const { execFileSync } = require("child_process");

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

  // theme vacio / invalido no debe reventar el navegador
  const r = rw.render({ seed: 1, theme: "", levels: 16, pixels: 320, width: 320, height: 180 });
  if (!r || !r.length) { console.error("  FALLO theme vacio devolvio nada"); fallos++; }
  else console.log("  OK   theme vacio elige uno al azar");

  console.log(fallos === 0 ? "\nTODO CORRECTO" : `\n${fallos} FALLOS`);
  process.exit(fallos === 0 ? 0 : 1);
})();