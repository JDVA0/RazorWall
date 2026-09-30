//go:build js && wasm

// Compilación para el navegador: expone el render a JavaScript para que la
// página del proyecto pueda generar wallpapers sin instalar nada.
//
// Se compila con:
//
//	GOOS=js GOARCH=wasm go build -o docs/razorwall.wasm .
//	cp "$(go env GOROOT)/lib/wasm/wasm_exec.js" docs/
//
// La API queda en window.razorwall. Ver docs/index.html.
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
	if !v.Get("seed").IsUndefined() {
		o.seed = int64(v.Get("seed").Int())
	}
	if !v.Get("theme").IsUndefined() {
		o.theme = v.Get("theme").String()
	}
	if !v.Get("levels").IsUndefined() {
		if n := v.Get("levels").Int(); n != 0 {
			o.levels = n
		}
	}
	if !v.Get("pixels").IsUndefined() {
		if n := v.Get("pixels").Int(); n != 0 {
			o.pixels = n
		}
	}
	if !v.Get("width").IsUndefined() {
		if n := v.Get("width").Int(); n != 0 {
			o.width = n
		}
	}
	if !v.Get("height").IsUndefined() {
		if n := v.Get("height").Int(); n != 0 {
			o.height = n
		}
	}
	if !v.Get("mirror").IsUndefined() {
		o.mirror = v.Get("mirror").Bool()
	}
	if !v.Get("smooth").IsUndefined() {
		o.smooth = v.Get("smooth").Bool()
	}
	return o
}

// renderPNG dibuja un fondo y lo devuelve codificado en PNG.
func renderPNG(o jsOptions) ([]byte, error) {
	rng := newRand(o.seed)
	theme := o.theme
	if theme == "" || theme == "random" {
		// "random" en el navegador se tira con Math.random, que es lo que
		// espera quien pulsa el botón
		theme = themeNames[int(jsRandom()*float64(len(themeNames)))]
	}

	spec := Spec{
		Seed:   o.seed,
		Theme:  theme,
		Levels: o.levels,
		Pixels: o.pixels,
		Width:  o.width,
		Height: o.height,
		Mirror: o.mirror,
		Smooth: o.smooth,
	}
	img, err := spec.Render(rng)
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

func main() {
	api := js.Global().Get("Object").New()

	api.Set("render", js.FuncOf(func(this js.Value, args []js.Value) any {
		var o jsOptions
		if len(args) > 0 {
			o = readOptions(args[0])
		}
		data, err := renderPNG(o)
		if err != nil {
			js.Global().Get("console").Call("error", "razorwall: "+err.Error())
			return js.Undefined()
		}
		out := js.Global().Get("Uint8Array").New(len(data))
		js.CopyBytesToJS(out, data)
		return out
	}))

	// catalogos, para que la pagina no los repita a mano
	api.Set("themes", toJSArray(themeNames))
	api.Set("bands", toJSArray(bandNames))
	api.Set("version", js.ValueOf(version))

	js.Global().Set("razorwall", api)

	if cb := js.Global().Get("onRazorwallReady"); cb.Type() == js.TypeFunction {
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
