//go:build js && wasm

// Compilación para el navegador: expone el render a JavaScript para que la
// página del proyecto pueda generar wallpapers sin instalar nada.
//
// Se compila con:
//
//	GOOS=js GOARCH=wasm go build -trimpath -ldflags="-s -w" -o docs/razor.wasm .
//
// El pegamento con el navegador es docs/wasm_exec.js, que tiene que ser el de
// la misma versión de Go que compiló el .wasm.
//
// Hay dos caminos:
//
//	render(opts)   devuelve el PNG entero de golpe. Es lo que usa la CLI por
//	               dentro, y lo que comparan los tests contra el binario.
//	begin/step/band   el mismo dibujo troceado, para que la página pueda ir
//	               pintando en vez de quedarse congelada mientras se calcula.
//
// La API queda en window.razor. Ver docs/index.html.
package main

import (
	"bytes"
	"image/png"
	"syscall/js"
)

// jsOptions es la lectura defensiva del objeto que recibe JavaScript. Todas
// las claves son opcionales: si falta una, se usa el valor por defecto en
// lugar de fallar, porque en el navegador el error se vería en la consola y
// no en ninguna parte útil.
type jsOptions struct {
	seed   int64
	theme  string
	levels int
	pixels int
	width  int
	height int
	mirror bool
	smooth bool
}

func readOptions(v js.Value) jsOptions {
	o := jsOptions{levels: 16, pixels: 320, width: 1920, height: 1080}
	if v.IsUndefined() || v.IsNull() {
		return o
	}
	get := v.Get
	if !get("seed").IsUndefined() {
		o.seed = int64(get("seed").Int())
	}
	if !get("theme").IsUndefined() {
		o.theme = get("theme").String()
	}
	if !get("levels").IsUndefined() {
		if n := get("levels").Int(); n != 0 {
			o.levels = n
		}
	}
	if !get("pixels").IsUndefined() {
		if n := get("pixels").Int(); n != 0 {
			o.pixels = n
		}
	}
	if !get("width").IsUndefined() {
		if n := get("width").Int(); n != 0 {
			o.width = n
		}
	}
	if !get("height").IsUndefined() {
		if n := get("height").Int(); n != 0 {
			o.height = n
		}
	}
	if !get("mirror").IsUndefined() {
		o.mirror = get("mirror").Bool()
	}
	if !get("smooth").IsUndefined() {
		o.smooth = get("smooth").Bool()
	}
	return o
}

// specFrom convierte las opciones de JavaScript en el Spec del núcleo,
// resolviendo el tema si viene vacío o como "random".
func specFrom(o jsOptions) Spec {
	theme := o.theme
	if theme == "" || theme == "random" {
		// "random" en el navegador se tira con Math.random, que es lo que
		// espera quien pulsa el botón
		theme = themeNames[int(jsRandom()*float64(len(themeNames)))]
	}
	return Spec{
		Seed:   o.seed,
		Theme:  theme,
		Levels: o.levels,
		Pixels: o.pixels,
		Width:  o.width,
		Height: o.height,
		Mirror: o.mirror,
		Smooth: o.smooth,
	}
}

// renderPNG dibuja un fondo entero y lo devuelve codificado en PNG.
func renderPNG(o jsOptions) ([]byte, error) {
	rng := newRand(o.seed)
	img, err := specFrom(o).Render(rng)
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	// Mismo encoder y mismo nivel de compresion que la CLI: asi la salida
	// del navegador es identica byte a byte a la del binario, y eso es lo
	// que comprueba test/wasm_test.cjs.
	if err := png.Encode(&buf, img); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// toJSArray convierte un []string de Go en un Array de JavaScript.
// js.ValueOf no acepta slices, hay que construir el array a mano.
func toJSArray(items []string) js.Value {
	arr := js.Global().Get("Array").New(len(items))
	for i, s := range items {
		arr.SetIndex(i, s)
	}
	return arr
}

// toJSBytes copia un []byte a un Uint8Array de JavaScript.
func toJSBytes(b []byte) js.Value {
	out := js.Global().Get("Uint8Array").New(len(b))
	js.CopyBytesToJS(out, b)
	return out
}

// job guarda el render en curso entre llamadas, porque el cálculo se
// trocea y JavaScript va pidiendo el siguiente paso.
var job *Progressive

func main() {
	api := js.Global().Get("Object").New()

	api.Set("render", js.FuncOf(func(this js.Value, args []js.Value) any {
		var o jsOptions
		if len(args) > 0 {
			o = readOptions(args[0])
		}
		data, err := renderPNG(o)
		if err != nil {
			js.Global().Get("console").Call("error", "razor: "+err.Error())
			return js.Undefined()
		}
		return toJSBytes(data)
	}))

	// --- camino troceado -----------------------------------------------
	api.Set("begin", js.FuncOf(func(this js.Value, args []js.Value) any {
		var o jsOptions
		if len(args) > 0 {
			o = readOptions(args[0])
		}
		spec := specFrom(o)
		j, err := NewProgressive(spec, newRand(o.seed))
		if err != nil {
			js.Global().Get("console").Call("error", "razor: "+err.Error())
			return js.Null()
		}
		job = j
		obj := js.Global().Get("Object").New()
		obj.Set("gridH", j.GridHeight())
		obj.Set("width", spec.Width)
		obj.Set("height", spec.Height)
		obj.Set("theme", spec.Theme)
		return obj
	}))

	api.Set("step", js.FuncOf(func(this js.Value, args []js.Value) any {
		if job == nil {
			return 0
		}
		n := 12
		if len(args) > 0 && !args[0].IsUndefined() {
			n = args[0].Int()
		}
		return job.Step(n)
	}))

	// band devuelve RGBA crudo, no un PNG: la pagina lo pega con
	// putImageData, que es mas rapido que decodificar un PNG por franja.
	api.Set("band", js.FuncOf(func(this js.Value, args []js.Value) any {
		if job == nil || len(args) < 2 {
			return js.Undefined()
		}
		img := job.Band(args[0].Int(), args[1].Int())
		return toJSBytes(img.Pix)
	}))

	// catalogos, para que la pagina no los repita a mano
	api.Set("themes", toJSArray(themeNames))
	api.Set("bands", toJSArray(bandNames))
	api.Set("version", js.ValueOf(version))

	js.Global().Set("razor", api)

	if cb := js.Global().Get("onRazorReady"); cb.Type() == js.TypeFunction {
		cb.Invoke()
	}

	// main no puede volver: si lo hace, el runtime se apaga y las funciones
	// de syscall/js dejan de responder. select{} bloquea para siempre.
	select {}
}

// jsRandom devuelve un float en [0,1) de Math.random.
func jsRandom() float64 {
	m := js.Global().Get("Math")
	if m.Type() != js.TypeObject {
		return 0
	}
	return m.Call("random").Float()
}
