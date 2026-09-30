// Nucleo de generacion: ruido Perlin, temas, cuantizacion de color y
// escalado. No toca el disco ni la terminal, asi que lo comparten sin
// cambios la CLI y la compilacion para WebAssembly.
package main

import (
	"fmt"
	"image"
	"math"
	"math/rand"
	"time"
)

// version identifica la build, en la CLI y en la pagina web.
const version = "1.0"

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

func lerp8(a, b uint8, t float64) uint8 {
	return uint8(math.Round(float64(a) + (float64(b)-float64(a))*t))
}

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

// fieldParams fija los parametros del campo de altura una sola vez.
//
// Separarlo del calculo permite llenar el campo por trozos, que es lo que
// usa la pagina web para no bloquear el navegador con un render largo. Los
// parametros se sortean aqui y no durante el relleno, para que todos los
// trozos salgan identicos entre si.
type fieldParams struct {
	octaves int
	gain    float64
	span    float64
	warp    float64
	ox, oy  float64
	seedOff int64
}

func newFieldParams(rng *rand.Rand, seed int64) fieldParams {
	return fieldParams{
		octaves: 4 + rng.Intn(4),           // 4..7
		gain:    0.45 + rng.Float64()*0.13, // 0.45..0.58
		// unidades de ruido a lo ancho: 2-5 da islas reconocibles
		span:    2.0 + rng.Float64()*2.5,
		warp:    rng.Float64() * 1.2,
		ox:      rng.Float64() * 400,
		oy:      rng.Float64() * 400,
		seedOff: seed + int64(rng.Intn(10000)),
	}
}

// heightmap genera el campo 0..1 con octavas de Perlin y domain warping.
func heightmap(w, h int, seed int64, rng *rand.Rand) []float64 {
	fp := newFieldParams(rng, seed)
	out := make([]float64, w*h)
	fp.fill(out, w, h, 0, h)
	normalize(out)
	return out
}

// fill calcula las filas [y0,y1) del campo dentro de out, que tiene que
// medir w*h. Se puede llamar por trozos; normalize() se aplica despues
// sobre el campo entero, porque los valores se estiran de forma global y
// hacerlo por partes daria un resultado distinto.
func (fp fieldParams) fill(out []float64, w, h, y0, y1 int) {
	if y0 < 0 {
		y0 = 0
	}
	if y1 > h {
		y1 = h
	}
	sx := fp.span / float64(w)
	sy := fp.span / float64(h)
	warpX := fp.warp * 2

	idx := y0 * w
	for y := y0; y < y1; y++ {
		baseY := float64(y)*sy + fp.oy
		for x := 0; x < w; x++ {
			baseX := float64(x)*sx + fp.ox
			var fx, fy float64
			if fp.warp > 0.01 {
				// El desplazamiento va a variables propias: aplicarlo
				// sobre baseX/baseY los contaminaria para el resto de
				// la fila y el ruido saldria a rayas verticales.
				wx := perlin(baseX*0.5+31, baseY*0.5+17, fp.seedOff+5, 3, 0.5)
				wy := perlin(baseX*0.5+11, baseY*0.5+47, fp.seedOff+9, 3, 0.5)
				fx = baseX + (wx-0.5)*warpX
				fy = baseY + (wy-0.5)*warpX
			} else {
				fx = baseX
				fy = baseY
			}
			out[idx] = perlin(fx, fy, fp.seedOff, fp.octaves, fp.gain)
			idx++
		}
	}
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
	return colorsFromHeights(heightmap(w, h, seed, rng), w, h, theme, levels, mirror)
}

// colorsFromHeights mapea un campo de altura ya normalizado a pixeles de
// color: rampa de bioma, dithering y cuantizacion.
func colorsFromHeights(heights []float64, w, h int, theme string, levels int, mirror bool) *image.RGBA {
	ramp := themes[theme]

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

// Spec describes one wallpaper to draw. It is the shared input of the CLI
// and of the WebAssembly build, so both produce exactly the same image
// from the same spec.
type Spec struct {
	Seed   int64
	Theme  string
	Levels int
	Pixels int
	Width  int
	Height int
	Mirror bool
	Smooth bool
}

// GridSize returns the size of the pixel grid for a spec, keeping the
// output aspect ratio. It is exported as a method because the browser
// build needs it to show the user how big the blocks will be.
func (s Spec) GridSize() (int, int) {
	w := s.Width
	if w <= 0 {
		w = defaultOutputWidth
	}
	h := s.Height
	if h <= 0 {
		h = defaultOutputHeight
	}
	pw := s.Pixels
	if pw < minGridWidth {
		pw = minGridWidth
	}
	ph := int(math.Round(float64(pw) * float64(h) / float64(w)))
	if ph%2 != 0 {
		ph++
	}
	if ph < minGridHeight {
		ph = minGridHeight
	}
	return pw, ph
}

const (
	defaultOutputWidth  = 1920
	defaultOutputHeight = 1080
	minGridWidth        = 64
	minGridHeight       = 64
)

// ResolveTheme returns the theme to use, picking one at random when the
// spec asks for it, and validating the name otherwise.
func (s Spec) ResolveTheme(rng *rand.Rand) (string, error) {
	name := s.Theme
	if name == "" || name == "random" {
		return themeNames[rng.Intn(len(themeNames))], nil
	}
	if _, ok := themes[name]; !ok {
		return "", fmt.Errorf("unknown theme %q", name)
	}
	return name, nil
}

// Render draws the wallpaper and returns it. It never touches the disk or
// the terminal: callers decide what to do with the pixels.
func (s Spec) Render(rng *rand.Rand) (*image.RGBA, error) {
	theme, err := s.ResolveTheme(rng)
	if err != nil {
		return nil, err
	}
	levels := s.Levels
	if _, ok := levelToneCounts[levels]; !ok {
		levels = 16
	}
	pw, ph := s.GridSize()
	grid := render(pw, ph, s.Seed, rng, theme, levels, s.Mirror)

	if s.Smooth {
		return scaleBilinear(grid, s.Width, s.Height), nil
	}
	return scalePointy(grid, s.Width, s.Height), nil
}

// levelToneCounts maps a tone count to its flag name, used to validate
// the --levels values the browser sends.
var levelToneCounts = map[int]string{4: "poster", 8: "retro", 16: "clasico", 32: "medio", 64: "suave"}

// ---------------------------------------------------------------------------
// Render progresivo
// ---------------------------------------------------------------------------
//
// La misma imagen, pero calculada en trozos para que quien la pinta pueda ir
// mostrando el resultado a medida que sale. El navegador bloquea su hilo
// mientras dura un render largo, asi que trocear el campo de altura (que es
// la parte cara) mantiene la pagina viva.
//
// El escalado se hace una sola vez con scalePointy o scaleBilinear, los
// mismos que usa la CLI: las bandas recortan ese resultado, de modo que no
// puede haber dos implementaciones que se separen.

type Progressive struct {
	spec     Spec
	fp       fieldParams
	heights  []float64
	grid     *image.RGBA
	scaled   *image.RGBA
	pw, ph   int
	outW     int
	outH     int
	rows     int
	finished bool
}

// NewProgressive prepara el trabajo y devuelve el numero de filas de campo
// que habra que calcular.
func NewProgressive(spec Spec, rng *rand.Rand) (*Progressive, error) {
	if _, err := spec.ResolveTheme(rng); err != nil {
		return nil, err
	}
	pw, ph := spec.GridSize()
	return &Progressive{
		fp:      newFieldParams(rng, spec.Seed),
		heights: make([]float64, pw*ph),
		pw:      pw,
		ph:      ph,
		outW:    spec.Width,
		outH:    spec.Height,
		spec:    spec,
	}, nil
}

// Step calcula hasta n filas mas del campo y devuelve cuantas lleva.
func (p *Progressive) Step(n int) int {
	if p.finished || p.rows >= p.ph {
		return p.rows
	}
	if n < 1 {
		n = 1
	}
	y1 := p.rows + n
	if y1 > p.ph {
		y1 = p.ph
	}
	p.fp.fill(p.heights, p.pw, p.ph, p.rows, y1)
	p.rows = y1
	return p.rows
}

// GridHeight es el numero de filas de campo que hay que calcular.
func (p *Progressive) GridHeight() int { return p.ph }

// Ready dice si el campo esta completo y ya se puede pintar.
func (p *Progressive) Ready() bool { return p.rows >= p.ph }

// finish normaliza el campo, lo colorea y lo escala una sola vez.
func (p *Progressive) finish() {
	if p.scaled != nil {
		return
	}
	normalize(p.heights)
	levels := p.spec.Levels
	if _, ok := levelToneCounts[levels]; !ok {
		levels = 16
	}
	p.grid = colorsFromHeights(p.heights, p.pw, p.ph, p.spec.Theme, levels, p.spec.Mirror)
	if p.spec.Smooth {
		p.scaled = scaleBilinear(p.grid, p.outW, p.outH)
	} else {
		p.scaled = scalePointy(p.grid, p.outW, p.outH)
	}
	p.finished = true
}

// Band devuelve las filas [y0,y1) de la imagen final, recortadas del
// resultado ya escalado.
func (p *Progressive) Band(y0, y1 int) *image.RGBA {
	if y0 < 0 {
		y0 = 0
	}
	if y1 > p.outH {
		y1 = p.outH
	}
	if y1 <= y0 {
		return image.NewRGBA(image.Rect(0, 0, p.outW, 0))
	}
	p.finish()
	out := image.NewRGBA(image.Rect(0, 0, p.outW, y1-y0))
	for y := y0; y < y1; y++ {
		copy(out.Pix[(y-y0)*out.Stride:], p.scaled.Pix[y*p.scaled.Stride:])
	}
	return out
}

// Image devuelve la imagen entera, calculando lo que falte.
func (p *Progressive) Image() *image.RGBA {
	p.finish()
	return p.scaled
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

// newRand returns the RNG used to draw a wallpaper. Keeping it here (and
// not calling rand.New directly at each call site) makes it easy to swap in
// a different source if the browser build ever needs one.
func newRand(seed int64) *rand.Rand { return rand.New(rand.NewSource(seed)) }
