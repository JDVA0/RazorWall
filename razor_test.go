//go:build !js

package main

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"image"
	"image/png"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// renderBytes dibuja un fondo y devuelve los bytes del PNG, para poder
// comparar dos renders sin tocar el disco.
func renderBytes(t *testing.T, seed int64, theme string, levels int, mirror bool) []byte {
	t.Helper()
	rng := rand.New(rand.NewSource(seed))
	grid := render(160, 90, seed, rng, theme, levels, mirror, -1)

	var buf bytes.Buffer
	if err := png.Encode(&buf, grid); err != nil {
		t.Fatalf("png.Encode: %v", err)
	}
	return buf.Bytes()
}

// TestDeterminismoEsElObjetivoPrincipal comprueba lo que el programa
// promete: misma semilla, mismo PNG byte a byte.
func TestDeterminismoEsElObjetivoPrincipal(t *testing.T) {
	for _, theme := range themeNames {
		a := renderBytes(t, 20260930, theme, 16, false)
		b := renderBytes(t, 20260930, theme, 16, false)
		if !bytes.Equal(a, b) {
			t.Errorf("tema %s: dos renders con la misma semilla difieren (%d vs %d bytes)",
				theme, len(a), len(b))
		}
	}
}

func TestSemillasDistintasDanImagenesDistintas(t *testing.T) {
	a := renderBytes(t, 1, "island", 16, false)
	b := renderBytes(t, 2, "island", 16, false)
	if bytes.Equal(a, b) {
		t.Error("semillas 1 y 2 produjeron la misma imagen")
	}
}

func TestMirrorCambiaLaImagen(t *testing.T) {
	a := renderBytes(t, 42, "island", 16, false)
	b := renderBytes(t, 42, "island", 16, true)
	if bytes.Equal(a, b) {
		t.Error("--mirror no cambió nada")
	}
}

func TestPerlinSeMantieneEnRango(t *testing.T) {
	for seed := int64(1); seed <= 8; seed++ {
		for i := 0; i < 400; i++ {
			x := rand.New(rand.NewSource(int64(i))).Float64() * 40
			y := rand.New(rand.NewSource(int64(i)+1)).Float64() * 40
			v := perlin(x, y, seed, 5, 0.5)
			if v < 0 || v > 1 {
				t.Fatalf("perlin fuera de rango con seed %d: %f", seed, v)
			}
		}
	}
}

func TestPerlinEnCoordenadasGrandes(t *testing.T) {
	// el flujo diario genera semillas como 20260930, que dan
	// coordenadas muy grandes; el ruido no debe romperse ahí
	v := perlin(202609.30, 2851.4, 20260930, 6, 0.5)
	if v < 0 || v > 1 {
		t.Errorf("perlin fuera de rango con coordenadas grandes: %f", v)
	}
}

func TestCuantizacionRedondeaATonosConocidos(t *testing.T) {
	for _, levels := range []int{4, 8, 16, 32, 64} {
		// los valores resultantes deben caer sobre la rejilla de tonos
		step := 255.0 / float64(levels-1)
		vistos := map[uint8]bool{}
		for c := 0; c <= 255; c++ {
			got := quantizeColor(uint8(c), levels)
			vistos[got] = true
			// comprueba que queda a menos de medio paso de la rejilla
			idx := math.Round(float64(got) / step)
			if math.Abs(float64(got)-idx*step) > 0.5*step+0.001 {
				t.Fatalf("levels=%d: c=%d produjo %d, fuera de la rejilla de tono %v",
					levels, c, got, step)
			}
		}
		if len(vistos) > levels {
			t.Errorf("levels=%d produjo %d tonos distintos, no puede ser", levels, len(vistos))
		}
	}
}

func TestTemaDelDiaNuncaRepiteAlDiaAnterior(t *testing.T) {
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	for i := 1; i < 400; i++ {
		hoy := themeForDay(base.AddDate(0, 0, i))
		ayer := themeForDay(base.AddDate(0, 0, i-1))
		if hoy == ayer {
			t.Fatalf("día %d: tema repetido (%s)", i, hoy)
		}
	}
}

func TestSampleRampDevuelveColorValido(t *testing.T) {
	for _, name := range themeNames {
		ramp := themes[name]
		if len(ramp) < 2 {
			t.Fatalf("tema %s: rampa demasiado corta", name)
		}
		for i := 0; i <= 100; i++ {
			r, g, b := sampleRamp(ramp, float64(i)/100)
			// sampleRamp no recorta; el recorte real es cosa de
			// quantizeColor, asi que solo comprobamos que no desborda
			// un canal entero
			if r > 255 || g > 255 || b > 255 || r < 0 || g < 0 || b < 0 {
				t.Fatalf("tema %s en t=%.2f devolvio (%d,%d,%d)", name, float64(i)/100, r, g, b)
			}
		}
	}
}

func TestEscaladoConservaLasDimensiones(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	grid := render(160, 90, 1, rng, "island", 16, false, -1)

	for _, sz := range []struct{ w, h int }{{1920, 1080}, {800, 600}, {640, 360}} {
		got := scalePointy(grid, sz.w, sz.h)
		if got.Bounds().Dx() != sz.w || got.Bounds().Dy() != sz.h {
			t.Errorf("scalePointy devolvio %dx%d, esperado %dx%d",
				got.Bounds().Dx(), got.Bounds().Dy(), sz.w, sz.h)
		}
	}
}

// TestEscaladoPointyDuplicaBloquesExactos es la propiedad que hace que el
// resultado sea pixel art de verdad: cada celda de la rejilla debe ocupar
// un rectangulo uniforme en la salida.
func TestEscaladoPointyDuplicaBloquesExactos(t *testing.T) {
	rng := rand.New(rand.NewSource(7))
	grid := render(160, 90, 7, rng, "neon", 16, false, -1)
	out := scalePointy(grid, 320, 180) // factor 2 exacto

	for y := 0; y < 180; y++ {
		for x := 0; x < 320; x++ {
			sx, sy := x/2, y/2
			want := grid.Pix[sy*grid.Stride+sx*4 : sy*grid.Stride+sx*4+3]
			got := out.Pix[y*out.Stride+x*4 : y*out.Stride+x*4+3]
			if !bytes.Equal(want, got) {
				t.Fatalf("pixel (%d,%d) no coincide con su celda origen (%d,%d)", x, y, sx, sy)
			}
		}
	}
}

// TestGenerateUsaEscaladoPointyPorDefecto vigila el cableado real, no solo
// la funcion aislada: si alguien cambia scalePointy por scaleBilinear dentro
// de generate(), este test lo nota aunque scalePointy siga siendo correcta.
func TestGenerateUsaEscaladoPointyPorDefecto(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "out.png")

	opt := options{
		theme:  "island",
		pixels: 160,
		levels: "clasico",
		width:  320,
		height: 180,
		format: "png",
	}
	if _, _, err := generate(opt, 99, out); err != nil {
		t.Fatalf("generate: %v", err)
	}

	f, err := os.Open(out)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer f.Close()
	decoded, err := png.Decode(f)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	img, ok := decoded.(*image.RGBA)
	if !ok {
		t.Fatalf("el PNG decodifico a %T, esperado *image.RGBA", decoded)
	}

	b := img.Bounds()
	at := func(x, y int) (uint32, uint32, uint32) {
		i := img.PixOffset(x, y)
		return uint32(img.Pix[i]), uint32(img.Pix[i+1]), uint32(img.Pix[i+2])
	}
	// factor 2 exacto: cada bloque 2x2 debe ser uniforme
	for by := 0; by < b.Dy(); by += 2 {
		for bx := 0; bx < b.Dx(); bx += 2 {
			r, g, bl := at(bx, by)
			for _, d := range [][2]int{{1, 0}, {0, 1}, {1, 1}} {
				r2, g2, b2 := at(bx+d[0], by+d[1])
				if r != r2 || g != g2 || bl != b2 {
					t.Fatalf("bloque en (%d,%d) no es uniforme: (%d,%d,%d) vs (%d,%d,%d). "+
						"El escalado por defecto deberia ser por vecino mas cercano",
						bx, by, r, g, bl, r2, g2, b2)
				}
			}
		}
	}
}

// TestRenderProgresivoCoincideConElCompleto es el que sostiene la pagina:
// si las franjas no dieran la misma imagen, el fondo que se ve mientras se
// genera seria distinto del que se acaba guardando.
func TestRenderProgresivoCoincideConElCompleto(t *testing.T) {
	casos := []Spec{
		{Seed: 20260930, Theme: "island", Levels: 16, Pixels: 160, Width: 640, Height: 360},
		{Seed: 7, Theme: "neon", Levels: 4, Pixels: 120, Width: 480, Height: 270},
		{Seed: 99, Theme: "volcano", Levels: 8, Pixels: 160, Width: 640, Height: 360, Mirror: true},
		{Seed: 555, Theme: "glacier", Levels: 32, Pixels: 160, Width: 640, Height: 360, Smooth: true},
	}
	for _, spec := range casos {
		completo, err := spec.Render(rand.New(rand.NewSource(spec.Seed)))
		if err != nil {
			t.Fatalf("Render(%s): %v", spec.Theme, err)
		}

		p, err := NewProgressive(spec, rand.New(rand.NewSource(spec.Seed)))
		if err != nil {
			t.Fatalf("NewProgressive(%s): %v", spec.Theme, err)
		}
		// el campo a trozos, como hara el navegador
		paso := 1 + p.GridHeight()/6
		for f := 0; f < p.GridHeight(); f += paso {
			p.Step(paso)
		}
		if !p.Ready() {
			t.Fatalf("%s: el campo no se dio por terminado", spec.Theme)
		}

		// y la imagen en franjas horizontales
		const franjas = 9
		alto := (spec.Height + franjas - 1) / franjas
		armado := image.NewRGBA(image.Rect(0, 0, spec.Width, spec.Height))
		for y := 0; y < spec.Height; y += alto {
			y1 := y + alto
			if y1 > spec.Height {
				y1 = spec.Height
			}
			banda := p.Band(y, y1)
			for dy := 0; dy < y1-y; dy++ {
				copy(armado.Pix[(y+dy)*armado.Stride:], banda.Pix[dy*banda.Stride:])
			}
		}
		if !bytes.Equal(armado.Pix, completo.Pix) {
			t.Errorf("tema %s: las franjas no coinciden con el render completo", spec.Theme)
		}
	}
}

func TestRenderProgresivoToleraBandasImprobables(t *testing.T) {
	spec := Spec{Seed: 5, Theme: "desert", Levels: 16, Pixels: 120, Width: 480, Height: 270}
	p, err := NewProgressive(spec, rand.New(rand.NewSource(5)))
	if err != nil {
		t.Fatal(err)
	}
	for p.Step(50) < p.GridHeight() {
	}
	casos := [][2]int{{-10, 40}, {200, 400}, {100, 100}, {0, 9999}}
	for _, c := range casos {
		b := p.Band(c[0], c[1])
		if b.Bounds().Dy() < 0 {
			t.Fatalf("banda %v dio alto negativo", c)
		}
		for i := 3; i < len(b.Pix); i += 4 {
			if b.Pix[i] != 0xFF {
				t.Fatalf("banda %v: pixel sin opacidad completa", c)
			}
		}
	}
}

func TestElRuidoCambiaLaEscalaDelTerreno(t *testing.T) {
	// ruido automatico (negativo) no debe fijarse a ningun valor
	auto := Spec{Seed: 42, Theme: "island", Levels: 16, Pixels: 160,
		Width: 640, Height: 360, Noise: -1}
	a, err := auto.Render(rand.New(rand.NewSource(42)))
	if err != nil {
		t.Fatal(err)
	}
	b, err := auto.Render(rand.New(rand.NewSource(42)))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(a.Pix, b.Pix) {
		t.Error("con ruido automatico dos renders de la misma semilla difieren")
	}

	// cada valor fijo tiene que dar algo distinto
	vistos := map[int]string{}
	for _, n := range []int{0, 50, 100} {
		s := Spec{Seed: 42, Theme: "island", Levels: 16, Pixels: 160,
			Width: 640, Height: 360, Noise: n}
		img, err := s.Render(rand.New(rand.NewSource(42)))
		if err != nil {
			t.Fatal(err)
		}
		h := sha256.Sum256(img.Pix)
		vistos[n] = fmt.Sprintf("%x", h[:8])
		// y tiene que ser reproducible
		img2, _ := s.Render(rand.New(rand.NewSource(42)))
		if !bytes.Equal(img.Pix, img2.Pix) {
			t.Errorf("ruido %d: dos renders difieren", n)
		}
	}
	if vistos[0] == vistos[50] || vistos[50] == vistos[100] {
		t.Error("el ruido no cambia la imagen")
	}
}

func TestRuidoFueraDeRangoSeAjusta(t *testing.T) {
	// 500 debe comportarse como 100, no reventar
	a, err := Spec{Seed: 1, Theme: "neon", Levels: 16, Pixels: 120,
		Width: 480, Height: 270, Noise: 500}.Render(rand.New(rand.NewSource(1)))
	if err != nil {
		t.Fatal(err)
	}
	b, err := Spec{Seed: 1, Theme: "neon", Levels: 16, Pixels: 120,
		Width: 480, Height: 270, Noise: 100}.Render(rand.New(rand.NewSource(1)))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(a.Pix, b.Pix) {
		t.Error("un ruido fuera de rango deberia equivale al maximo")
	}
}

func TestGuardarPNGRespetaElTamanoPedido(t *testing.T) {
	dir := t.TempDir()
	rng := rand.New(rand.NewSource(3))
	grid := render(64, 36, 3, rng, "desert", 8, false, -1)
	out := filepath.Join(dir, "test.png")

	if err := save(grid, out, "png"); err != nil {
		t.Fatalf("save: %v", err)
	}
	f, err := os.Open(out)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer f.Close()
	img, err := png.Decode(f)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if img.Bounds().Dx() != 64 || img.Bounds().Dy() != 36 {
		t.Errorf("PNG guardado mide %v, esperado 64x36", img.Bounds().Size())
	}
}

func TestParseSize(t *testing.T) {
	casos := []struct {
		in      string
		w, h    int
		fallara bool
	}{
		{"1920x1080", 1920, 1080, false},
		{"2560X1440", 2560, 1440, false},
		{"abc", 0, 0, true},
		{"1920", 0, 0, true},
		{"0x0", 0, 0, true},
		{"-1x10", 0, 0, true},
	}
	for _, c := range casos {
		w, h, err := parseSize(c.in)
		if c.fallara {
			if err == nil {
				t.Errorf("parseSize(%q) deberia fallar", c.in)
			}
			continue
		}
		if err != nil {
			t.Errorf("parseSize(%q) fallo: %v", c.in, err)
		}
		if w != c.w || h != c.h {
			t.Errorf("parseSize(%q) = %dx%d, esperado %dx%d", c.in, w, h, c.w, c.h)
		}
	}
}

func TestTodosLosNivelesProducenImagenValida(t *testing.T) {
	for name := range levelNames {
		rng := rand.New(rand.NewSource(5))
		grid := render(80, 45, 5, rng, "island", levelNames[name], false, -1)
		if grid.Bounds().Dx() != 80 || grid.Bounds().Dy() != 45 {
			t.Fatalf("nivel %s: rejilla con tamaño inesperado", name)
		}
		// cada pixel debe tener opacidad completa
		for i := 3; i < len(grid.Pix); i += 4 {
			if grid.Pix[i] != 0xFF {
				t.Fatalf("nivel %s: pixel opaco esperado, obtuve %d", name, grid.Pix[i])
			}
		}
	}
}

func TestImagenTieneVariacionDeColor(t *testing.T) {
	// un render completamente plano seria un bug silencioso
	rng := rand.New(rand.NewSource(11))
	grid := render(160, 90, 11, rng, "island", 16, false, -1)
	vistos := map[uint32]bool{}
	for i := 0; i < len(grid.Pix); i += 4 {
		vistos[uint32(grid.Pix[i])<<16|uint32(grid.Pix[i+1])<<8|uint32(grid.Pix[i+2])] = true
	}
	if len(vistos) < 8 {
		t.Errorf("solo %d colores distintos: el render parece plano", len(vistos))
	}
}
