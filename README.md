<img src="logo.svg" width="420" alt="Razor">

Generador de wallpapers pixel art a partir de ruido Perlin.

> La herramienta se llama **Razor** (`razor`). Este repositorio es
> **RazorWall**, el nombre del proyecto.

Cada día produce un fondo distinto usando la fecha como semilla, así que el
escritorio no se repite. El fondo de cualquier día pasado se puede volver a
generar cuando quieras.

Todo el programa está en Go y no depende de nada: el PNG se escribe con la
librería estándar, sin Python ni ImageMagick. El mismo motor también compila
para WebAssembly, así que hay una [versión web](#en-el-navegador) que no
necesita instalar nada.

![Fondo generado por Razor](images/hero.png)

## Índice

- [Instalación](#instalación)
- [Uso rápido](#uso-rápido)
- [Opciones](#opciones)
- [Catálogo visual](#catálogo-visual)
  - [Temas](#temas)
  - [Niveles de color](#niveles-de-color)
  - [Resolución de la rejilla](#resolución-de-la-rejilla)
  - [Escala del ruido](#escala-del-ruido)
  - [Proporción de la imagen](#proporción-de-la-imagen)
  - [Simetría](#simetría)
  - [Pixel art frente a interpolado](#pixel-art-frente-a-interpolado)
  - [Modo aleatorio](#modo-aleatorio)
  - [Galería](#galería)
- [En el navegador](#en-el-navegador)
- [Poner el fondo como wallpaper](#poner-el-fondo-como-wallpaper)
- [Cómo funciona](#cómo-funciona)
- [Desarrollo](#desarrollo)
- [Licencia](#licencia)

## Instalación

Requiere Go 1.21 o superior.

```bash
git clone https://github.com/JDVA0/RazorWall.git
cd RazorWall
go build -ldflags="-s -w" -o razor .
```

El binario resultante ocupa unos 2 MB. Para probarlo sin instalar nada,
[descarga uno de los binarios](#descargas) o usa `go run`:

```bash
go run . --theme volcano
```

## Uso rápido

Sin argumentos, `razor` **muestra la ayuda y no genera nada**. En cuanto le
pasas alguna opción, dibuja el fondo:

```bash
./razor                        # muestra la ayuda
./razor --theme neon           # el fondo de hoy en tema neón
./razor --random --preview     # algo distinto cada vez, en la terminal
./razor --gallery 8            # ocho variaciones para elegir
```

La imagen se guarda **en la carpeta donde está el binario**, con el nombre de la
fecha (`2026-09-30.png`), sin importar desde qué directorio lo ejecutes. Usa
`--dir` o `--out` si quieres otro sitio.

## Opciones

| Opción | Qué hace |
| --- | --- |
| `-t`, `--theme` | Tema de color: `island`, `volcano`, `desert`, `glacier`, `jungle`, `neon` o `random` |
| `-x`, `--pixels` | Ancho de la rejilla pixelada. Cuanto menor, bloques más grandes (por defecto `320`) |
| `--levels` | Tonos por canal: `suave` 64, `medio` 32, `clasico` 16, `retro` 8, `poster` 4 |
| `--size` | Tamaño final de la imagen, p. ej. `2560x1440` (por defecto `1920x1080`) |
| `--quality` | Atajo de resolución: `fast` 1280x720, `normal` 1920x1080, `high` 2560x1440 |
| `--mirror` | Refleja el terreno en horizontal |
| `--noise` | Escala del terreno, de 0 a 100. Por defecto la sortea la semilla |
| `-p`, `--preview` | Dibuja también el resultado en la terminal, a color |
| `-r`, `--random` | Tira tema, niveles, rejilla y resolución al azar |
| `-N`, `--seed` | Semilla manual (por defecto, la fecha del día) |
| `--date` | Usa otra fecha: `--date 2026-01-15` |
| `--yesterday`, `--tomorrow` | El fondo del día anterior o del siguiente |
| `-o`, `--out` | Ruta exacta del archivo de salida |
| `-d`, `--dir` | Carpeta de salida (por defecto, la del propio binario) |
| `--format` | `png` (por defecto) o `jpg` |
| `--smooth` | Escala con interpolación en vez de vecino más cercano |
| `-g`, `--gallery` | Genera N variaciones en una subcarpeta `gallery` |
| `-l`, `--list` | Lista estilos, temas y niveles |
| `--version` | Muestra la versión |

Las opciones admiten las dos formas: `--theme volcano` y `--theme=volcano`.

### Ejemplos

```bash
# Tema cálido y bien saturado
./razor --theme volcano --levels retro

# Sorpresa: tema, tamaño, rejilla y resolución al azar, visto en la terminal
./razor --random --preview

# Un fondo concreto y reproducible
./razor --seed 42 --theme island --pixels 480

# Composición simétrica a 1440p
./razor --mirror --quality high --out /tmp/paisaje.png

# Ocho candidatos para elegir el mejor
./razor --gallery 8
```

### Cómo se comporta `--random`

Las opciones que escribas explícitamente **ganan** sobre el azar. Con
`--random --theme neon` obtienes tema neón fijo y el resto aleatorio.

El nombre del archivo incluye lo que salió, para que las tiradas no se pisen
entre sí:

```
2026-09-30-neon-320-retro-ffih.png
          │    │   │     │    └─ semilla
          │    │   │     └────── niveles
          │    │   └──────────── rejilla
          │    └──────────────── tema
          └───────────────────── fecha
```

## Catálogo visual

Cada opción tiene su propia imagen, generada con la semilla `20260930` para
que las comparaciones sirvan. Haz clic en cualquiera para verla a tamaño
completo.

### Temas

Cada tema define su propia rampa de bioma, del abismo a la cumbre. Se
generan con `./razor --theme <tema>`.

<table>
<tr>
<td width="50%"><img src="images/themes/island.png"><br><sub><b>island</b> — océano profundo, bajíos, playa y cumbres nevadas</sub></td>
<td width="50%"><img src="images/themes/volcano.png"><br><sub><b>volcano</b> — basalte y roca oscura con vetas de lava</sub></td>
</tr>
<tr>
<td><img src="images/themes/desert.png"><br><sub><b>desert</b> — dunas y cañones áridos, sin agua</sub></td>
<td><img src="images/themes/glacier.png"><br><sub><b>glacier</b> — mar abierto, hielo y roca desnuda</sub></td>
</tr>
<tr>
<td><img src="images/themes/jungle.png"><br><sub><b>jungle</b> — selva densa y riscos entre la vegetación</sub></td>
<td><img src="images/themes/neon.png"><br><sub><b>neon</b> — synthwave entre cian, magenta y ámbar</sub></td>
</tr>
</table>

Si no indicas `--theme`, el tema rota a diario pasando por los seis y nunca
repite el del día anterior:

```
2026-09-28  neon
2026-09-29  island
2026-09-30  volcano
2026-10-01  desert
2026-10-02  glacier
2026-10-03  jungle
2026-10-04  neon
```

### Niveles de color

Cuántos tonos distintos se usan por canal. Con muchos tonos el degradado es
casi continuo; con pocos, cada zona queda como un plano de color uniforme.
Todas estas imágenes usan la misma semilla y la misma rejilla (`--pixels 200`),
así que la única variable es el color.

<table>
<tr>
<td width="33%"><img src="images/levels/suave.png"><br><sub><b>suave</b> — 64 tonos/canal. Apenas sugiere el pixel art</sub></td>
<td width="33%"><img src="images/levels/medio.png"><br><sub><b>medio</b> — 32 tonos/canal</sub></td>
<td width="33%"><img src="images/levels/clasico.png"><br><sub><b>clasico</b> — 16 tonos/canal (por defecto)</sub></td>
</tr>
<tr>
<td><img src="images/levels/retro.png"><br><sub><b>retro</b> — 8 tonos/canal, aspecto de Game Boy</sub></td>
<td><img src="images/levels/poster.png"><br><sub><b>poster</b> — 4 tonos/canal, un plano por zona</sub></td>
<td></td>
</tr>
</table>

### Resolución de la rejilla

`--pixels` fija el ancho de la rejilla sobre la que se calcula el ruido. La
rejilla se amplía después al tamaño final con vecino más cercano, así que **cada
celda se convierte en un bloque nítido**. El valor es, por tanto, el lado del
bloque en píxeles de la imagen final.

<table>
<tr>
<td width="33%"><img src="images/pixels/120.png"><br><sub><b>--pixels 120</b> — bloques de 16 px, casi mosaico</sub></td>
<td width="33%"><img src="images/pixels/200.png"><br><sub><b>--pixels 200</b> — bloques de 10 px, aspecto Game Boy</sub></td>
<td width="33%"><img src="images/pixels/320.png"><br><sub><b>--pixels 320</b> — bloques de 6 px (por defecto)</sub></td>
</tr>
<tr>
<td><img src="images/pixels/480.png"><br><sub><b>--pixels 480</b> — bloques de 4 px</sub></td>
<td><img src="images/pixels/640.png"><br><sub><b>--pixels 640</b> — bloques de 3 px</sub></td>
<td></td>
</tr>
</table>

El tiempo de render depende de `--pixels`, no de la resolución de salida: subir
la rejilla de 120 a 960 multiplica el coste por unas cinco veces (0,05 s →
0,24 s), mientras que pasar la salida de 1920x1080 a 2560x1440 apenas lo
altera, porque el cálculo ocurre siempre en la rejilla pequeña.

### Escala del ruido

`--noise` fija el tamaño de las formas del terreno. En `0` salen islas grandes y
tranquilas; en `100`, un terreno muy picado. Sin el flag, cada día elige uno
distinto a partir de la semilla, así que el mismo día siempre da lo mismo.

<table>
<tr>
<td width="33%"><img src="images/noise/0.png" width="320"><br><sub><b>--noise 0</b> — islas grandes</sub></td>
<td width="33%"><img src="images/noise/50.png" width="320"><br><sub><b>--noise 50</b> — escala media</sub></td>
<td width="33%"><img src="images/noise/100.png" width="320"><br><sub><b>--noise 100</b> — muy picado</sub></td>
</tr>
</table>

El valor solo sustituye a la escala: el resto de parámetros (cuántas octavas, el
warping, dónde cae la semilla) sigue viniendo de la semilla, así que mover el
deslizador no baraja nada más de la imagen.

### Proporción de la imagen

`--size` acepta cualquier proporción. La rejilla mantiene la relación del
ancho, así que el terreno se estira verticalmente en formatos más altos:

<table>
<tr>
<td><img src="images/size/21x9-2560x1080.png"><br><sub>21:9 ultrawide</sub></td>
<td><img src="images/size/16x9-1920x1080.png"><br><sub>16:9 (por defecto)</sub></td>
</tr>
<tr>
<td><img src="images/size/9x16-1080x1920.png"><br><sub>9:16 vertical</sub></td>
<td><img src="images/size/1x1-1200x1200.png"><br><sub>1:1 cuadrado</sub></td>
</tr>
</table>

### Simetría

`--mirror` refleja el terreno en el eje vertical. Queda más equilibrado como
fondo de pantalla que el ruido puro, y resulta cómodo si vas a poner iconos
encima. Misma semilla en las dos:

<table>
<tr>
<td width="50%"><img src="images/mirror/off.png"><br><sub>sin <code>--mirror</code></sub></td>
<td width="50%"><img src="images/mirror/on.png"><br><sub>con <code>--mirror</code></sub></td>
</tr>
</table>

### Pixel art frente a interpolado

Por defecto el escalado usa vecino más cercano, que es lo que produce los
bloques duros. Con `--smooth` se usa interpolación bilineal y el resultado se
ve como una imagen normal, sin carácter pixel art.

<table>
<tr>
<td width="50%"><img src="images/smooth/off.png"><br><sub>por defecto: bloques nítidos</sub></td>
<td width="50%"><img src="images/smooth/on.png"><br><sub><code>--smooth</code>: bloques difuminados</sub></td>
</tr>
</table>

*Para que la diferencia se aprecie, estas dos imágenes usan una rejilla de 40
píxeles.*

### Modo aleatorio

`--random` tira tema, niveles, rejilla y resolución, y usa entropía real del
sistema en vez de la fecha, así que cada llamada da algo distinto. Seis
ejecuciones seguidas:

<table>
<tr>
<td width="33%"><img src="images/random/01.png"></td>
<td width="33%"><img src="images/random/02.png"></td>
<td width="33%"><img src="images/random/03.png"></td>
</tr>
<tr>
<td><img src="images/random/04.png"></td>
<td><img src="images/random/05.png"></td>
<td><img src="images/random/06.png"></td>
</tr>
</table>

### Galería

`--gallery N` genera N variaciones a partir de la misma fecha, con semillas
separadas por 7919, en una subcarpeta `gallery`:

<table>
<tr>
<td width="25%"><img src="images/gallery/01.png"><br><sub>0001.png</sub></td>
<td width="25%"><img src="images/gallery/02.png"><br><sub>0002.png</sub></td>
<td width="25%"><img src="images/gallery/03.png"><br><sub>0003.png</sub></td>
<td width="25%"><img src="images/gallery/04.png"><br><sub>0004.png</sub></td>
</tr>
</table>

## En el navegador

Hay una versión web en [`docs/`](docs/), publicada en GitHub Pages. Muestra el
fondo a pantalla completa y lo genera en el equipo, con el mismo algoritmo que
el binario.

No hay paneles: abajo hay una sola barra negra con el selector de tema y los
deslizadores, y el resto se hace con el ratón o el teclado.

El selector de tema tiene los seis temas y un botón de **al azar** que elige uno.
Los botones se construyen con los nombres que publica el motor, así que la
página no lleva su propia copia de la lista. Cuando el tema cambia por otra vía
—un clic en el fondo o las teclas `1` a `6`— el botón correspondiente se
marca solo.

<table>
<tr>
<td width="33%"><b>clic</b> o <b>espacio</b><br><sub>fondo nuevo al azar</sub></td>
<td width="33%"><b>arrastrar</b><br><sub>mueve el fondo, sin generar otro</sub></td>
<td width="33%"><b>s</b><br><sub>descarga el PNG</sub></td>
</tr>
<tr>
<td><b>1</b> … <b>6</b><br><sub>tema exacto</sub></td>
<td><b>Mayús</b><br><sub>otra semilla, mismo tema</sub></td>
<td><b>doble clic</b> o <b>0</b><br><sub>vuelve a centrar</sub></td>
</tr>
<tr>
<td><b>rueda</b><br><sub>zoom en el cursor</sub></td>
<td><b>m</b> / <b>i</b><br><sub>reflejo / interpolación</sub></td>
<td><b>[</b> / <b>]</b><br><sub>menos / más tonos</sub></td>
</tr>
</table>

Los cuatro deslizadores controlan los píxeles de la rejilla, los niveles de
color, la escala del ruido y el zoom. El zoom no vuelve a generar la imagen: solo la amplía, así que
responde al instante.

### Se ve cómo se forma

El fondo no aparece de golpe. El motor expone el dibujo en dos fases y la página
las va pidiendo:

- `begin(opts)` deja el trabajo preparado y dice cuántas filas de campo faltan.
- `step(n)` calcula `n` filas del campo de altura, que es la parte cara.
- `band(y0, y1)` devuelve las franjas ya coloreadas, como RGBA crudo.

Entre una llamada y la siguiente se cede el hilo al navegador, así que el fondo
se ve crecer por franjas horizontales y la página no se congela a media
generación. Si mueves un deslizador mientras se dibuja, el render en curso se
abandona y empieza el nuevo.

La descarga sale del propio lienzo con `toBlob`, así que no hace falta generar
nada por segunda vez.

Las franjas tienen que dar **exactamente** la misma imagen que el render de una
pasada. Eso lo comprueban `TestRenderProgresivoCoincideConElCompleto` y las
bandas de `wasm_test.cjs`, porque si no, el fondo que ves mientras se genera
sería distinto del que se guarda.

### Compilar la versión web

```bash
GOOS=js GOARCH=wasm go build -trimpath -ldflags="-s -w" -o docs/razor.wasm .
```

`wasm_exec.js` va versionado en `docs/` y hay que regenerarlo **a la vez** que
el `.wasm`: es el pegamento entre Go y el navegador y tiene que ser de la misma
versión de Go que lo compiló. La ruta cambia según la versión, así que conviene
buscarla en las dos:

```bash
GOOS=js GOARCH=wasm go build -trimpath -ldflags="-s -w" -o docs/razor.wasm .
for p in lib/wasm misc/wasm; do
  [ -f "$(go env GOROOT)/$p/wasm_exec.js" ] && cp "$(go env GOROOT)/$p/wasm_exec.js" docs/ && break
done
```

Si cambias la versión de Go y solo regeneras uno de los dos, la página se queda
sin arrancar.

La página **necesita servirse por http**, no vale con abrir el archivo a doble
clic: los navegadores bloquean la lectura de un `.wasm` desde `file://`. Si lo
abres en local y ves el aviso, es por eso.

## Poner el fondo como wallpaper

`razor` no toca la configuración de tu escritorio: genera la imagen y
nada más. Eso es deliberado, para que la misma herramienta sirva en cualquier
sistema sin inventariar gestores de ventanas.

Fija el resultado con la herramienta que ya uses:

```bash
# GNOME
gsettings set org.gnome.desktop.background picture-uri "file://$PWD/2026-09-30.png"

# KDE Plasma
plasma-apply-wallpaper 2026-09-30.png

# Xfce
xfconf-query -c xfce4-desktop -p /backdrop/workspace0/last-image -s "$PWD/2026-09-30.png"

# X11 con feh
feh --bg-fill 2026-09-30.png
```

Si prefieres automatizarlo, un `cron` diario que ejecute `razor` y tu
comando de escritorio es suficiente:

```cron
17 6 * * * cd ~/RazorWall && ./razor --random && feh --bg-fill "$(ls -t *.png | head -1)"
```

## Cómo funciona

1. **La fecha es la semilla.** `2026-09-30` se convierte en `20260930`. Mismo
   día, mismo fondo, siempre.

2. **Campo de altura con Perlin.** Se suman 4 a 7 octavas de ruido Perlin
   clásico (gradientes aleatorios, no ruido de valor), con curva de suavizado
   quintica. Encima se aplica *domain warping*, que desplaza las coordenadas de
   muestreo con otro ruido: así las costas se curvan y no salen círculos ni
   manchas.

3. **Bandas de bioma.** La altura se corta en franjas (abismo, mar, bajío,
   playa, campo, bosque, roca, cumbre) según la rampa del tema, interpolando el
   color entre franjas.

4. **Dithering ordenado.** Una matriz de Bayer 4x4 rompe los bordes rectos entre
   franjas sin introducir ruido aleatorio, de modo que el resultado sigue siendo
   reproducible.

5. **Cuantización.** El color se reduce a N tonos por canal. Esto es lo que da el
   aspecto pixel art.

6. **Escalado.** La rejilla (320x180 por defecto) se amplía al tamaño final.

### Sobre `--preview`

Con `--preview` el fondo se dibuja en la terminal con bloques medios (`▀`) en
color de 24 bits: cada carácter representa dos píxeles verticales, el de arriba
en primer plano y el de abajo en segundo plano. Necesitas una terminal con
color verdadero; si la salida está redirigida a un archivo, se verá en blanco y
negro.

## Desarrollo

### Compilar y probar

```bash
go build -o razor .
go test ./...                     # núcleo: determinismo, ruido, escalado, CLI
node test/wasm_test.cjs           # el wasm frente a la CLI, y las franjas
node test/page_test.cjs           # el JS de docs/index.html con un DOM simulado
gofmt -l .                        # debe salir vacío
go vet ./...
```

Los tests de Go comprueban, entre otras cosas, que la misma semilla produzca
siempre un PNG idéntico byte a byte, que semillas distintas den imágenes
distintas, que el ruido se mantenga en rango y que la rotación diaria de temas
no repita.

`wasm_test.cjs` carga el módulo real de WebAssembly en Node y compara su salida
con la del binario para los mismos parámetros: **tienen que coincidir byte a
byte**. Si divergen, la web y la CLI ya no son el mismo generador.

`page_test.cjs` ejecuta el script real de la página contra un DOM mínimo y
comprueba que el cableado funciona: que cada control llega a `razor.render`
con lo que corresponde, que arrastrar mueve sin regenerar y que el zoom no
vuelve a dibujar.

### Estructura

```
render.go         núcleo: ruido Perlin, temas, cuantización, escalado y
                  el render progresivo por franjas
razor.go          CLI: banner, lectura de argumentos y escritura de archivos
wasm.go           bindings para el navegador (syscall/js)
razor_test.go     tests del núcleo y de la CLI
test/             pruebas del wasm y de la página (Node)
docs/             la página web, el logo y el .wasm compilado
images/           capturas del catálogo visual
```

El núcleo no toca el disco ni la terminal, y por eso lo comparten sin cambios la
CLI y el navegador. Hay dos entradas: `Spec.Render` para el render de una
pasada, que usa la CLI, y `Progressive` para el troceado, que usa la página.
Ambas comparten el mismo campo de altura y las mismas funciones de escalado, así
que no pueden dar imágenes distintas.

El logo ASCII del banner está incrustado como constante `logoLines`, así que el
fuente no depende de ningún archivo externo.

## Licencia

MIT. Ver [LICENSE](LICENSE).