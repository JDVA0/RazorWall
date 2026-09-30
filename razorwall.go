//go:build !js

// RazorWall: generador de wallpapers pixel art con ruido Perlin.
//
// La fecha se convierte en semilla, asi que cada dia produce un fondo
// distinto y cualquier dia pasado se puede regenerar con --date.
//
// La parte que dibuja el fondo vive en render.go, compartida con la
// compilacion para WebAssembly de wasm.go.
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

// ---------------------------------------------------------------------------
// Marca
// ---------------------------------------------------------------------------

// logo ASCII incrustado como constante para que el fuente sea autonomo:
// no depende de ningun archivo externo. Si lo cambias, actualiza aqui.
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
//
// The drawing itself is delegated to Spec.Render, the same entry point the
// WebAssembly build uses, so the CLI and the browser cannot drift apart.
func generate(opt options, seed int64, out string) (string, float64, error) {
	rng := rand.New(rand.NewSource(seed))

	levels, ok := levelNames[opt.levels]
	if !ok {
		levels = 16
	}
	theme, err := opt.spec(seed, levels).ResolveTheme(rng)
	if err != nil {
		return "", 0, fmt.Errorf("%w (see --list)", err)
	}

	spec := opt.spec(seed, levels)
	spec.Theme = theme
	pw, ph := spec.GridSize()

	fmt.Printf("  pixel grid %dx%d -> %dx%d   theme %s, %d levels\n",
		pw, ph, spec.Width, spec.Height, theme, levels)

	start := time.Now()

	if opt.preview {
		// the preview draws the low-res grid, so render it separately
		// instead of paying for the full-size upscale twice
		printPreview(render(pw, ph, seed, rng, theme, levels, opt.mirror), 72)
	}

	img, err := spec.Render(rng)
	if err != nil {
		return "", 0, err
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

// spec converts parsed command line options into a Spec for the renderer.
func (o options) spec(seed int64, levels int) Spec {
	return Spec{
		Seed:   seed,
		Theme:  o.theme,
		Levels: levels,
		Pixels: o.pixels,
		Width:  o.width,
		Height: o.height,
		Mirror: o.mirror,
		Smooth: o.smooth,
	}
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
