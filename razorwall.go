// Command razorwall generates pixel art wallpapers from Perlin noise.
//
// The date is turned into a seed, so every day produces a different
// wallpaper and any past day can be regenerated on demand.
// No external dependencies: PNG is written with the image/png stdlib.
package main

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const version = "1.0"

// ---------------------------------------------------------------------------
// Marca
// ---------------------------------------------------------------------------

// logo incrustado desde logo.txt para que el fuente sea un solo archivo.
const logoLines = "" +
	":::::::..    :::.     :::::::::    ...    :::::::..\n" +
	";;;;``;;;;   ;;`;;    '`````;;; .;;;;;;;. ;;;;``;;;;\n" +
	" [[[,/[[['  ,[[ '[[,      .n[[',[[     \\[[,[[[,/[[['\n" +
	" $$$$$$c   c$$$cc$$$c   ,$$P\"  $$$,     $$$$$$$$$c\n" +
	" 888b \"88bo,888   888,,888bo,_ \"888,_ _,88P888b \"88bo,\n" +
	" MMMM   \"W\" YMM   \"\"`  `\"\"*UMM   \"YMMMMMP\" MMMM   \"W\""

type stop struct {
	t       float64
	r, g, b uint8
}

// Rampa morada: de violeta profundo a lavanda.
var purpleRamp = []stop{
	{0.00, 0x2A, 0x0F, 0x4A},
	{0.25, 0x5B, 0x21, 0xA5},
	{0.50, 0x8B, 0x3F, 0xD4},
	{0.75, 0xC0, 0x7B, 0xF0},
	{1.00, 0xE9, 0xD5, 0xFF},
}

func purpleAt(t float64) (uint8, uint8, uint8) {
	if t < 0 {
		t = 0
	} else if t > 1 {
		t = 1
	}
	for i := 0; i < len(purpleRamp)-1; i++ {
		a, b := purpleRamp[i], purpleRamp[i+1]
		if t <= b.t {
			k := (t - a.t) / (b.t - a.t)
			return lerp8(a.r, b.r, k), lerp8(a.g, b.g, k), lerp8(a.b, b.b, k)
		}
	}
	last := purpleRamp[len(purpleRamp)-1]
	return last.r, last.g, last.b
}

func lerp8(a, b uint8, t float64) uint8 {
	return uint8(math.Round(float64(a) + (float64(b)-float64(a))*t))
}

// hasUnicodePrintf indica si la salida soporta secuencias ANSI de 24 bits.
var colorEnabled = detectColor()

func detectColor() bool {
	fi, err := os.Stdout.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice != 0
}

func printBanner() {
	lines := strings.Split(logoLines, "\n")
	maxW := 0
	for _, l := range lines {
		if len([]rune(l)) > maxW {
			maxW = len([]rune(l))
		}
	}
	pad := (78 - maxW) / 2
	if pad < 0 {
		pad = 0
	}
	indent := strings.Repeat(" ", pad)

	for y, line := range lines {
		base := 0.0
		if len(lines) > 1 {
			base = float64(y) / float64(len(lines)-1)
		}
		if !colorEnabled {
			fmt.Println(indent + line)
			continue
		}
		var b strings.Builder
		for _, ch := range line {
			// el glifo hace de mascara: mas tinta, mas brillo
			lum := math.Min(1, float64(ch%97)/60)
			t := lum*0.65 + base*0.35
			r, g, bl := purpleAt(t)
			fmt.Fprintf(&b, "\033[38;2;%d;%d;%dm%c\033[0m", r, g, bl, ch)
		}
		fmt.Println(indent + b.String())
	}

	sub := "procedural pixel art wallpapers"
	line := strings.Repeat("-", 78)
	fmt.Println(line)
	if colorEnabled {
		r, g, b := purpleAt(0.8)
		fmt.Printf("\033[38;2;%d;%d;%dm%s\033[0m\n", r, g, b, line)
		fmt.Printf("\033[2m%s\033[0m\n", center(sub, len(line)))
	} else {
		fmt.Println(line)
		fmt.Println(center(sub, len(line)))
	}
}

func center(s string, width int) string {
	if len(s) >= width {
		return s
	}
	left := (width - len(s)) / 2
	return strings.Repeat(" ", left) + s
}

// ---------------------------------------------------------------------------
// Utilidades numericas
// ---------------------------------------------------------------------------

func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

func lerp(a, b, t float64) float64 { return a + (b-a)*t }

func quantizeColor(c uint8, levels int) uint8 {
	if levels >= 256 {
		return c
	}
	if levels <= 1 {
		return 0
	}
	step := 255.0 / float64(levels-1)
	q := math.Round(float64(c)/step) * step
	return uint8(math.Round(clamp01(q/255) * 255))
}

// ---------------------------------------------------------------------------
// Ruido Perlin
// ---------------------------------------------------------------------------

// perlin2 cachea la permutacion por semilla: se usa mucho dentro de los
// bucles y barajarla cada llamada seria carisimo.
var perms = map[int64]*[512]int{}

func permutation(seed int64) *[512]int {
	if p, ok := perms[seed]; ok {
		return p
	}
	r := rand.New(rand.NewSource(seed))
	p := new([512]int)
	for i := 0; i < 256; i++ {
		p[i] = i
	}
	r.Shuffle(256, func(i, j int) { p[i], p[j] = p[j], p[i] })
	copy(p[256:], p[:256])
	if len(perms) > 64 {
		perms = map[int64]*[512]int{}
	}
	perms[seed] = p
	return p
}

var grads = [8][2]float64{
	{1, 1}, {-1, 1}, {1, -1}, {-1, -1},
	{1, 0}, {-1, 0}, {0, 1}, {0, -1},
}

func grad(h int, x, y float64) float64 {
	g := grads[h&7]
	return g[0]*x + g[1]*y
}

// perlinOctave es una octava de Perlin en 2D, devuelve 0..1.
func perlinOctave(x, y float64, seed int64) float64 {
	perm := permutation(seed)
	xi := int(math.Floor(x))
	yi := int(math.Floor(y))
	xf := x - float64(xi)
	yf := y - float64(yi)

	// curva quintica: la que usa el Perlin original, evita artefactos
	u := xf * xf * xf * (xf*(xf*6-15) + 10)
	v := yf * yf * yf * (yf*(yf*6-15) + 10)

	xi8 := xi & 255
	yi8 := yi & 255
	xi1 := (xi + 1) & 255
	yi1 := (yi + 1) & 255

	aa := perm[perm[xi8]+yi8]
	ab := perm[perm[xi8]+yi1]
	ba := perm[perm[xi1]+yi8]
	bb := perm[perm[xi1]+yi1]

	x1 := lerp(grad(aa, xf, yf), grad(ba, xf-1, yf), u)
	x2 := lerp(grad(ab, xf, yf-1), grad(bb, xf-1, yf-1), u)
	// +-0.5 es el rango teorico; se desplaza para quedar en 0..1
	return lerp(x1, x2, v) + 0.5
}

// perlin suma octavas y normaliza a 0..1.
func perlin(x, y float64, seed int64, octaves int, gain float64) float64 {
	total, amp, norm := 0.0, 1.0, 0.0
	fx, fy := x, y
	for o := 0; o < octaves; o++ {
		total += perlinOctave(fx, fy, seed+int64(o)*1013) * amp
		norm += amp
		amp *= gain
		fx *= 2
		fy *= 2
	}
	return total / norm
}

// ---------------------------------------------------------------------------
// Temas de bioma
// ---------------------------------------------------------------------------

type band struct {
	h    float64
	r, g uint8
	b    uint8
}

var themes = map[string][]band{
	"island": {
		{0.00, 0x0A, 0x1E, 0x3A}, // deep water
		{0.34, 0x12, 0x3A, 0x63}, // sea
		{0.44, 0x2E, 0x7C, 0xA8}, // shallows
		{0.47, 0xD8, 0xC9, 0x8A}, // sand
		{0.50, 0x5E, 0x8C, 0x3A}, // grass
		{0.62, 0x2F, 0x5D, 0x28}, // trees
		{0.78, 0x77, 0x77, 0x6E}, // rock
		{0.90, 0xF0, 0xF4, 0xF8}, // snow
	},
	"volcano": {
		{0.00, 0x1A, 0x08, 0x0C},
		{0.36, 0x4A, 0x11, 0x12},
		{0.46, 0x2A, 0x1A, 0x1E},
		{0.50, 0x4A, 0x3A, 0x36}, // basalt
		{0.58, 0x6B, 0x4A, 0x32},
		{0.72, 0x8A, 0x6A, 0x44},
		{0.84, 0xE8, 0x5C, 0x1E}, // lava
		{0.93, 0xFF, 0xE0, 0x6B}, // hot lava
	},
	"desert": {
		{0.00, 0x2A, 0x1E, 0x3C},
		{0.30, 0x8A, 0x5A, 0x3A},
		{0.45, 0xC4, 0x92, 0x4E},
		{0.52, 0xE8, 0xC4, 0x7A}, // dunes
		{0.66, 0xB8, 0x86, 0x48},
		{0.80, 0x8C, 0x5C, 0x30},
		{0.92, 0xF0, 0xE4, 0xC0},
	},
	"glacier": {
		{0.00, 0x06, 0x14, 0x28},
		{0.32, 0x10, 0x36, 0x5E},
		{0.46, 0x3A, 0x76, 0xA8},
		{0.49, 0xD6, 0xEC, 0xF6},
		{0.56, 0xE8, 0xF4, 0xFA}, // ice
		{0.74, 0xB8, 0xD4, 0xE4},
		{0.88, 0x6A, 0x7A, 0x92}, // bare rock
		{0.95, 0xFF, 0xFF, 0xFF},
	},
	"jungle": {
		{0.00, 0x04, 0x1A, 0x1A},
		{0.32, 0x0E, 0x4A, 0x46},
		{0.45, 0x1E, 0x7A, 0x62},
		{0.49, 0x4A, 0x8C, 0x4E},
		{0.60, 0x27, 0x5A, 0x2A}, // trees
		{0.74, 0x1A, 0x3E, 0x1E},
		{0.88, 0x6E, 0x6A, 0x5E}, // cliffs
	},
	"neon": {
		{0.00, 0x08, 0x04, 0x1C},
		{0.32, 0x2A, 0x0A, 0x5E},
		{0.44, 0x7A, 0x14, 0xB8},
		{0.48, 0xFF, 0x2E, 0x9A},
		{0.56, 0x18, 0xE0, 0xE0},
		{0.72, 0x9A, 0x1E, 0xF0},
		{0.88, 0xFF, 0xD0, 0x40},
	},
}

var themeNames = []string{"island", "volcano", "desert", "glacier", "jungle", "neon"}

var bandNames = []string{"abyss", "sea", "shallows", "beach", "field", "forest", "rock", "summit"}

var levelNames = map[string]int{
	"suave": 64, "medio": 32, "clasico": 16, "retro": 8, "poster": 4,
}

func levelNamesSorted() []string {
	return []string{"suave", "medio", "clasico", "retro", "poster"}
}

// sampleRamp interpola el color de la rampa de bioma.
func sampleRamp(ramp []band, t float64) (uint8, uint8, uint8) {
	if t <= ramp[0].h {
		return ramp[0].r, ramp[0].g, ramp[0].b
	}
	prev := ramp[0]
	for _, cur := range ramp {
		if t <= cur.h {
			span := cur.h - prev.h
			k := 0.0
			if span > 1e-9 {
				k = (t - prev.h) / span
			}
			return lerp8(prev.r, cur.r, k), lerp8(prev.g, cur.g, k), lerp8(prev.b, cur.b, k)
		}
		prev = cur
	}
	last := ramp[len(ramp)-1]
	return last.r, last.g, last.b
}

// dithering ordenado 4x4: rompe los bordes rectos entre bandas sin
// introducir ruido aleatorio, asi que el resultado sigue siendo
// reproducible.
var bayer4 = [16]int{
	0, 8, 2, 10,
	12, 4, 14, 6,
	3, 11, 1, 9,
	15, 7, 13, 5,
}

const ditherAmt = 0.035

// ---------------------------------------------------------------------------
// Campo de altura
// ---------------------------------------------------------------------------

// heightmap genera el campo 0..1 con octavas de Perlin y domain warping.
func heightmap(w, h int, seed int64, rng *rand.Rand) []float64 {
	octaves := 4 + rng.Intn(4)        // 4..7
	gain := 0.45 + rng.Float64()*0.13 // 0.45..0.58
	// unidades de ruido a lo ancho: 2-5 da islas reconocibles
	span := 2.0 + rng.Float64()*2.5
	warp := rng.Float64() * 1.2
	ox := rng.Float64() * 400
	oy := rng.Float64() * 400
	seedOff := seed + int64(rng.Intn(10000))

	sx := span / float64(w)
	sy := span / float64(h)
	warpX := warp * 2

	out := make([]float64, w*h)
	idx := 0
	for y := 0; y < h; y++ {
		baseY := float64(y)*sy + oy
		for x := 0; x < w; x++ {
			baseX := float64(x)*sx + ox
			var fx, fy float64
			if warp > 0.01 {
				// El desplazamiento va a variables propias: aplicarlo
				// sobre baseX/baseY los contaminaria para el resto de
				// la fila y el ruido saldria a rayas verticales.
				wx := perlin(baseX*0.5+31, baseY*0.5+17, seedOff+5, 3, 0.5)
				wy := perlin(baseX*0.5+11, baseY*0.5+47, seedOff+9, 3, 0.5)
				fx = baseX + (wx-0.5)*warpX
				fy = baseY + (wy-0.5)*warpX
			} else {
				fx, fy = baseX, baseY
			}
			out[idx] = perlin(fx, fy, seedOff, octaves, gain)
			idx++
		}
	}

	normalize(out)
	return out
}

// normalize estira los valores a 0..1 para que las bandas salgan repartidas.
func normalize(v []float64) {
	lo, hi := v[0], v[0]
	for _, x := range v {
		if x < lo {
			lo = x
		}
		if x > hi {
			hi = x
		}
	}
	if hi-lo < 1e-9 {
		for i := range v {
			v[i] = 0.5
		}
		return
	}
	inv := 1 / (hi - lo)
	for i := range v {
		v[i] = (v[i] - lo) * inv
	}
}

// ---------------------------------------------------------------------------
// Render
// ---------------------------------------------------------------------------

// render dibuja el campo de altura como pixel art en la rejilla w x h.
func render(w, h int, seed int64, rng *rand.Rand, theme string, levels int, mirror bool) *image.RGBA {
	ramp := themes[theme]
	heights := heightmap(w, h, seed, rng)

	if mirror {
		// Horizontal mirror: the terrain becomes symmetrical, which
		// usually reads better as a wallpaper than raw noise.
		for y := 0; y < h; y++ {
			row := y * w
			for x := 0; x < w/2; x++ {
				a := row + x
				b := row + (w - 1 - x)
				heights[a], heights[b] = heights[b], heights[a]
			}
		}
	}

	img := image.NewRGBA(image.Rect(0, 0, w, h))
	// p recorre pixeles; idx recorre bytes RGBA (4 por pixel)
	p, idx := 0, 0
	for y := 0; y < h; y++ {
		brow := (y & 3) * 4
		for x := 0; x < w; x++ {
			t := heights[p]
			t += (float64(bayer4[brow+(x&3)])/16 - 0.5) * ditherAmt
			r, g, b := sampleRamp(ramp, clamp01(t))
			img.Pix[idx] = quantizeColor(r, levels)
			img.Pix[idx+1] = quantizeColor(g, levels)
			img.Pix[idx+2] = quantizeColor(b, levels)
			img.Pix[idx+3] = 0xFF
			p++
			idx += 4
		}
	}
	return img
}

// themeForDay picks the theme of the day, making sure it differs from
// yesterday's so a long streak of identical looking mornings is avoided.
func themeForDay(d time.Time) string {
	n := len(themeNames)
	y := d.Year()
	doy := d.YearDay()
	// mix the year in so themes shift between years
	idx := (y*31 + doy) % n
	prev := (y*31 + doy - 1) % n
	if idx == prev {
		idx = (idx + 1) % n
	}
	return themeNames[idx]
}

// printPreview draws the low-res grid in the terminal using half blocks,
// so each character cell shows two vertical pixels in true colour.
func printPreview(grid *image.RGBA, cols int) {
	sw := grid.Bounds().Dx()
	sh := grid.Bounds().Dy()
	rows := cols * sh / sw / 2
	if rows < 1 {
		rows = 1
	}
	if rows > 60 {
		rows = 60
	}
	var b strings.Builder
	// each character row covers step source rows, sampled at top and
	// bottom of the cell
	step := sh / rows
	if step < 1 {
		step = 1
	}
	for ry := 0; ry < rows; ry++ {
		b.Reset()
		y0 := ry * step
		if y0 >= sh {
			y0 = sh - 1
		}
		y1 := y0 + step - 1
		if y1 >= sh {
			y1 = sh - 1
		}
		for rx := 0; rx < cols; rx++ {
			x := rx * sw / cols
			if x >= sw {
				x = sw - 1
			}
			o0 := y0*grid.Stride + x*4
			o1 := y1*grid.Stride + x*4
			fmt.Fprintf(&b, "\033[38;2;%d;%d;%dm\033[48;2;%d;%d;%dm▀",
				grid.Pix[o0], grid.Pix[o0+1], grid.Pix[o0+2],
				grid.Pix[o1], grid.Pix[o1+1], grid.Pix[o1+2])
		}
		b.WriteString("\033[0m")
		fmt.Println(b.String())
	}
}

// scalePointy escala con vecino mas cercano: cada pixel de la rejilla se
// convierte en un bloque nitido, que es el efecto pixel art.
func scalePointy(src *image.RGBA, w, h int) *image.RGBA {
	sw := src.Bounds().Dx()
	sh := src.Bounds().Dy()
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		sy := y * sh / h
		if sy >= sh {
			sy = sh - 1
		}
		row := src.Pix[sy*src.Stride : sy*src.Stride+sw*4]
		for x := 0; x < w; x++ {
			sx := x * sw / w
			if sx >= sw {
				sx = sw - 1
			}
			copy(dst.Pix[y*dst.Stride+x*4:], row[sx*4:sx*4+4])
		}
	}
	return dst
}

// scaleBilinear escala con interpolacion, para cuando no se quiere el
// aspecto pixel art.
func scaleBilinear(src *image.RGBA, w, h int) *image.RGBA {
	sb := src.Bounds()
	sw, sh := sb.Dx(), sb.Dy()
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	fx := float64(sw) / float64(w)
	fy := float64(sh) / float64(h)
	for y := 0; y < h; y++ {
		sy := (float64(y)+0.5)*fy - 0.5
		y0 := int(math.Floor(sy))
		ty := sy - float64(y0)
		y0c, y1c := clampInt(y0, 0, sh-1), clampInt(y0+1, 0, sh-1)
		for x := 0; x < w; x++ {
			sx := (float64(x)+0.5)*fx - 0.5
			x0 := int(math.Floor(sx))
			tx := sx - float64(x0)
			x0c, x1c := clampInt(x0, 0, sw-1), clampInt(x0+1, 0, sw-1)
			o := y*dst.Stride + x*4
			for c := 0; c < 3; c++ {
				p00 := float64(src.Pix[y0c*src.Stride+x0c*4+c])
				p10 := float64(src.Pix[y0c*src.Stride+x1c*4+c])
				p01 := float64(src.Pix[y1c*src.Stride+x0c*4+c])
				p11 := float64(src.Pix[y1c*src.Stride+x1c*4+c])
				v := lerp(lerp(p00, p10, tx), lerp(p01, p11, tx), ty)
				dst.Pix[o+c] = uint8(clampInt(int(v+0.5), 0, 255))
			}
			dst.Pix[o+3] = 0xFF
		}
	}
	return dst
}

func clampInt(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// ---------------------------------------------------------------------------
// Escritura
// ---------------------------------------------------------------------------

func save(img image.Image, path, format string) error {
	if dir := filepath.Dir(path); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	bw := bufio.NewWriter(f)
	var e error
	switch format {
	case "jpg", "jpeg":
		e = jpeg.Encode(bw, img, &jpeg.Options{Quality: 92})
	default:
		e = png.Encode(bw, img)
	}
	if e != nil {
		return e
	}
	return bw.Flush()
}

// ---------------------------------------------------------------------------
// Configuracion
// ---------------------------------------------------------------------------

type options struct {
	theme    string
	pixels   int
	levels   string
	width    int
	height   int
	seed     int64
	hasSeed  bool
	out      string
	dir      string
	format   string
	smooth   bool
	random   bool
	mirror   bool
	preview  bool
	gallery  int
	list     bool
	showVer  bool
	dayShift int
	dateArg  string
}

type outputSize struct{ w, h int }

var qualitySizes = map[string]outputSize{
	"fast":   {1280, 720},
	"normal": {1920, 1080},
	"high":   {2560, 1440},
}

var defaultSize = outputSize{1920, 1080}

const outputFolder = "wallpapers"

// writableDir returns the first of dirs where a test file can be created,
// or "." if none works. Used to decide where to drop the PNG.
func writableDir(dirs ...string) string {
	for _, d := range dirs {
		if d == "" {
			continue
		}
		if err := os.MkdirAll(d, 0o755); err != nil {
			continue
		}
		probe := filepath.Join(d, ".razorwall-write-test")
		f, err := os.Create(probe)
		if err != nil {
			continue
		}
		f.Close()
		os.Remove(probe)
		abs, err := filepath.Abs(d)
		if err != nil {
			return d
		}
		return abs
	}
	return "."
}

// baseDir decides where the PNG is written.
//
// Default is the directory holding the executable: running `razorwall`
// from anywhere drops the wallpaper next to the program instead of
// scattering files in the working directory. If that location is not
// writable (a binary installed in /usr/bin, say) it falls back to the
// working directory.
func baseDir(override string) string {
	if override != "" {
		return override
	}
	exeDir := ""
	if exe, err := os.Executable(); err == nil {
		exeDir = filepath.Dir(exe)
	}
	return writableDir(exeDir, ".")
}

func defaultWallpaperPath(dir string, day time.Time, format string) string {
	return filepath.Join(dir, day.Format("2006-01-02")+"."+format)
}

func galleryDir(dir string) string { return filepath.Join(dir, "gallery") }

// generate renders one wallpaper. It returns the path it wrote to.
func generate(opt options, seed int64, out string) (string, float64, error) {
	rng := rand.New(rand.NewSource(seed))

	levels, ok := levelNames[opt.levels]
	if !ok {
		levels = 16
	}
	theme := opt.theme
	if theme == "" || theme == "random" {
		theme = themeNames[rng.Intn(len(themeNames))]
	}
	if _, valid := themes[theme]; !valid {
		return "", 0, fmt.Errorf("unknown theme %q (see --list)", theme)
	}

	// pixel grid, keeping the output aspect ratio
	pw := opt.pixels
	if pw < 64 {
		pw = 64
	}
	ph := int(math.Round(float64(pw) * float64(opt.height) / float64(opt.width)))
	if ph%2 != 0 {
		ph++
	}
	if ph < 64 {
		ph = 64
	}

	fmt.Printf("  pixel grid %dx%d -> %dx%d   theme %s, %d levels\n",
		pw, ph, opt.width, opt.height, theme, levels)

	start := time.Now()
	grid := render(pw, ph, seed, rng, theme, levels, opt.mirror)

	if opt.preview {
		printPreview(grid, 72)
	}

	var img image.Image
	if opt.smooth {
		img = scaleBilinear(grid, opt.width, opt.height)
	} else {
		img = scalePointy(grid, opt.width, opt.height)
	}

	if out == "" {
		out = defaultWallpaperPath(".", time.Now(), opt.format)
	}
	base := strings.TrimSuffix(out, filepath.Ext(out))
	out = base + "." + opt.format

	if err := save(img, out, opt.format); err != nil {
		return "", 0, err
	}

	elapsed := time.Since(start)
	kb := fileSizeKB(out)
	fmt.Printf("  %-8s %-10s %-8s %6.0fms  %d KB\n", "perlin", theme, opt.levels,
		float64(elapsed.Microseconds())/1000, kb)
	fmt.Printf("  \033[2m-> %s\033[0m\n", out)
	return out, elapsed.Seconds(), nil
}

func fileSizeKB(path string) int {
	fi, err := os.Stat(path)
	if err != nil {
		return 0
	}
	return int(fi.Size() / 1024)
}

// ---------------------------------------------------------------------------
// CLI
// ---------------------------------------------------------------------------

func printList() {
	fmt.Println("Styles (1)")
	fmt.Println("  perlin    pixel art terrain from Perlin noise")
	fmt.Println()
	fmt.Printf("Themes: %s\n", strings.Join(themeNames, ", "))
	fmt.Printf("Biome bands: %s\n", strings.Join(bandNames, ", "))
	fmt.Println()
	fmt.Println("Levels (--levels):")
	fmt.Printf("  %s\n", levelListText())
}

func levelListText() string {
	parts := make([]string, 0, len(levelNames))
	for _, k := range levelNamesSorted() {
		parts = append(parts, fmt.Sprintf("%s=%d tones/channel", k, levelNames[k]))
	}
	return strings.Join(parts, ", ")
}

const usage = `razorwall - pixel art wallpapers from Perlin noise

Usage:
  razorwall [options]

Run with no arguments to render today's wallpaper next to this binary,
named YYYY-MM-DD.png. Same day, same picture, every time.

Options:
  -r, --random           surprise me: random theme, levels, grid
                         and resolution
  -t, --theme NAME       color theme, or 'random'
  -x, --pixels N         pixel grid width (default 320)
      --levels LEVEL     tones per channel (default clasico)
      --size WxH         final image size (default 1920x1080)
      --quality LEVEL    fast | normal | high
      --mirror           mirror the terrain horizontally
  -p, --preview          also draw it in the terminal
  -N, --seed N           manual seed (default: today's date)
      --date YYYY-MM-DD  pretend it is another day
      --yesterday        yesterday's wallpaper
      --tomorrow         tomorrow's wallpaper
  -o, --out PATH         output file
  -d, --dir PATH         output folder (default: this binary's folder)
      --format FMT       png (default) or jpg
      --smooth           scale with interpolation instead of pixel art
  -g, --gallery N        render N variations into a gallery subfolder
  -l, --list             list styles, themes and levels, then exit
      --version          show the version and exit
  -h, --help             this help

Levels:  %s
Themes:  %s

Examples:
  razorwall --theme volcano --levels retro
  razorwall --random --preview
  razorwall --seed 42 --pixels 480
  razorwall --gallery 8
`

// printQuickDocs is the no-argument landing page.
func printQuickDocs() {
	printBanner()
	fmt.Println()
	fmt.Printf(usage, strings.Join(levelNamesSorted(), "/"),
		strings.Join(themeNames, ", "))
}

func fail(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, "Error: "+format+"\n", args...)
	os.Exit(2)
}

// entropySeed returns a fresh seed for --random, seeded from the system
// RNG rather than from the date.
func entropySeed() int64 {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return time.Now().UnixNano()
	}
	return int64(binary.LittleEndian.Uint64(b[:]) >> 1)
}

// shortTag renders a seed as a short lowercase tag, used to keep
// successive --random files from overwriting one another.
func shortTag(seed int64) string {
	const alphabet = "abcdefghijkmnpqrstuvwxyz23456789"
	v := uint64(seed)
	out := make([]byte, 4)
	for i := 3; i >= 0; i-- {
		out[i] = alphabet[v%uint64(len(alphabet))]
		v /= uint64(len(alphabet))
	}
	return string(out)
}

func main() {
	opt := options{
		pixels: 320,
		levels: "clasico",
		format: "png",
		width:  defaultSize.w,
		height: defaultSize.h,
	}
	// Which knobs the user set explicitly; --random only rolls the rest.
	var hasExplicitLevels, hasExplicitPixels, hasExplicitSize bool

	args := os.Args[1:]
	// need returns the argument following the option at idx.
	need := func(idx int, flag string) string {
		if idx >= len(args) {
			fail("%s needs a value", flag)
		}
		return args[idx]
	}

	for i := 0; i < len(args); i++ {
		a := args[i]
		var val string
		var haveVal bool
		if eq := strings.IndexByte(a, '='); eq > 0 && strings.HasPrefix(a, "-") {
			val, haveVal = a[eq+1:], true
			a = a[:eq]
		}
		// take devuelve el valor de la opcion. Se pasa el indice por
		// puntero porque desde Go 1.22 cada iteracion del bucle tiene su
		// propia copia de i, asi que una closure que lo incrementara no
		// avanzaria el bucle real.
		take := func(idx *int) string {
			if haveVal {
				return val
			}
			*idx++
			return need(*idx, a)
		}

		switch a {
		case "-t", "--theme":
			opt.theme = take(&i)
		case "-x", "--pixels":
			n, err := strconv.Atoi(take(&i))
			if err != nil {
				fail("--pixels needs a number")
			}
			opt.pixels = n
			hasExplicitPixels = true
		case "--levels":
			opt.levels = take(&i)
			hasExplicitLevels = true
		case "--size":
			w, h, err := parseSize(take(&i))
			if err != nil {
				fail("%v", err)
			}
			opt.width, opt.height = w, h
			hasExplicitSize = true
		case "--quality":
			q := take(&i)
			sz, ok := qualitySizes[q]
			if !ok {
				fail("--quality must be fast, normal or high")
			}
			opt.width, opt.height = sz.w, sz.h
			hasExplicitSize = true
		case "-N", "--seed":
			n, err := strconv.ParseInt(take(&i), 10, 64)
			if err != nil {
				fail("--seed needs an integer")
			}
			opt.seed, opt.hasSeed = n, true
		case "--date":
			opt.dateArg = take(&i)
		case "--yesterday":
			opt.dayShift = -1
		case "--tomorrow":
			opt.dayShift = 1
		case "-o", "--out":
			opt.out = take(&i)
		case "-d", "--dir":
			opt.dir = take(&i)
		case "--format":
			opt.format = take(&i)
		case "--smooth":
			opt.smooth = true
		case "--mirror":
			opt.mirror = true
		case "-p", "--preview":
			opt.preview = true
		case "-r", "--random":
			opt.random = true
		case "-g", "--gallery":
			n, err := strconv.Atoi(take(&i))
			if err != nil {
				fail("--gallery needs a number")
			}
			opt.gallery = n
		case "-l", "--list":
			opt.list = true
		case "--version":
			opt.showVer = true
		case "-h", "--help":
			printQuickDocs()
			return
		default:
			fail("unknown option %q (see --help)", a)
		}
	}

	// No arguments at all: show the docs instead of writing a file.
	if len(args) == 0 {
		printQuickDocs()
		return
	}

	if opt.showVer {
		fmt.Printf("RazorWall %s\n", version)
		return
	}
	if opt.list {
		printBanner()
		fmt.Println()
		printList()
		return
	}
	if opt.format != "png" && opt.format != "jpg" && opt.format != "jpeg" {
		fail("--format must be png or jpg")
	}

	printBanner()

	// date -> seed
	day := time.Now().AddDate(0, 0, opt.dayShift)
	if opt.dateArg != "" {
		parsed, err := time.Parse("2006-01-02", opt.dateArg)
		if err != nil {
			fail("--date must be YYYY-MM-DD")
		}
		day = parsed
	}
	baseSeed := int64(day.Year()*10000 + int(day.Month())*100 + day.Day())
	seed := baseSeed
	if opt.hasSeed {
		seed = opt.seed
	}
	dayName := day.Format("2006-01-02")

	// --random rolls theme, quantisation, grid size and resolution in one
	// go. Explicit flags still win, so `--random --theme neon` keeps neon
	// and randomises the rest.
	if opt.random {
		if !opt.hasSeed {
			// The date seed is deterministic, so rolling from it would
			// give the same picture every run. Random mode needs real
			// entropy, hence a fresh seed per invocation.
			seed = entropySeed()
		}
		roll := rand.New(rand.NewSource(seed))
		if opt.theme == "" {
			opt.theme = themeNames[roll.Intn(len(themeNames))]
		}
		if !hasExplicitLevels {
			keys := levelNamesSorted()
			opt.levels = keys[roll.Intn(len(keys))]
		}
		if !hasExplicitPixels {
			opt.pixels = []int{160, 200, 240, 320, 400, 480}[roll.Intn(6)]
		}
		if !hasExplicitSize {
			sizes := []outputSize{{1280, 720}, {1920, 1080}, {2560, 1440}, {3840, 2160}}
			s := sizes[roll.Intn(len(sizes))]
			opt.width, opt.height = s.w, s.h
		}
	} else if opt.theme == "" || opt.theme == "random" && !opt.hasSeed {
		// Daily pick: cycle through themes but never repeat yesterday's.
		opt.theme = themeForDay(day)
	}

	outDir := baseDir(opt.dir)

	if opt.gallery > 0 {
		fmt.Printf("\n  date %s   seed %d   output %s\n", dayName, seed, outDir)
		fmt.Printf("  rendering %d variations...\n", opt.gallery)
		start := time.Now()
		for i := 0; i < opt.gallery; i++ {
			gopt := opt
			gopt.out = filepath.Join(galleryDir(outDir), fmt.Sprintf("%04d.%s", i+1, opt.format))
			s := seed + int64(i)*7919
			if _, _, err := generate(gopt, s, gopt.out); err != nil {
				fail("%v", err)
			}
		}
		fmt.Printf("\n  done in %.1fs\n", time.Since(start).Seconds())
		return
	}

	out := opt.out
	if out == "" {
		out = defaultWallpaperPath(outDir, day, opt.format)
		if opt.random {
			// record what was rolled plus a tag from the seed, so
			// successive runs do not overwrite each other
			base := strings.TrimSuffix(filepath.Base(out), filepath.Ext(out))
			out = filepath.Join(outDir, fmt.Sprintf("%s-%s-%d-%s-%s.%s",
				base, opt.theme, opt.pixels, opt.levels, shortTag(seed),
				opt.format))
		}
	}

	if opt.random {
		fmt.Printf("\n  random pick: theme %s, %s, grid %d, %dx%d, seed %d\n\n",
			opt.theme, opt.levels, opt.pixels, opt.width, opt.height, seed)
	} else {
		fmt.Printf("\n  date %s   seed %d   style perlin\n\n", dayName, seed)
	}

	if _, _, err := generate(opt, seed, out); err != nil {
		fail("%v", err)
	}
}

func parseSize(s string) (int, int, error) {
	parts := strings.Split(strings.ToLower(s), "x")
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("expected WIDTHxHEIGHT, e.g. 1920x1080")
	}
	w, err1 := strconv.Atoi(parts[0])
	h, err2 := strconv.Atoi(parts[1])
	if err1 != nil || err2 != nil || w <= 0 || h <= 0 {
		return 0, 0, fmt.Errorf("expected WIDTHxHEIGHT, e.g. 1920x1080")
	}
	return w, h, nil
}
