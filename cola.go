package main

// Cola de conversión: cada imagen pasa por el motor WebP oficial de Google (cwebp).
//
// Máxima calidad con el menor peso, sin que el usuario elija nada:
//   - JPG (fotos): WebP con pérdida a calidad 92, con diferencia invisible a la vista.
//   - PNG: se codifica sin pérdida y con pérdida a la vez, y se queda con la mejor.
//     Las capturas, logos y gráficos salen sin pérdida (idénticos píxel a píxel y más livianos);
//     las fotos guardadas como PNG, con pérdida a calidad 92 (pesan muchísimo menos).

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
)

const (
	calidadFoto = "92"
	// Un PNG sale sin pérdida si pesa como mucho un 20 % más que con pérdida.
	margenSinPerdida = 1.2
	// Hasta este tamaño se vuelve a comprimir sin pérdida al máximo (-z 9): más lento, ~10 % menos.
	pixelesCompresionMaxima = 8_000_000
)

type Item struct {
	ID      int      `json:"id"`
	Nombre  string   `json:"nombre"`  // nombre del archivo original
	Salida  string   `json:"salida"`  // nombre del .webp
	Tamano  string   `json:"tamano"`  // original | 1:1 | 16:9 | 9:16
	Estado  string   `json:"estado"`  // cola | convirtiendo | lista | error
	Antes   int64    `json:"antes"`   // bytes del original
	Despues int64    `json:"despues"` // bytes del WebP
	Modo    string   `json:"modo"`    // sin-perdida | calidad-alta
	Ancho   int      `json:"ancho"`
	Alto    int      `json:"alto"`
	Archivo string   `json:"archivo"` // ruta completa del .webp
	Error   *Mensaje `json:"error"`

	entrada string // copia temporal del original
	formato string // png | jpg
}

type Cola struct {
	mu      sync.Mutex
	items   []*Item
	next    int
	app     *App
	trabajo chan *Item
	ctx     context.Context
	cancel  context.CancelFunc
}

func NuevaCola(app *App) *Cola {
	ctx, cancel := context.WithCancel(context.Background())
	q := &Cola{app: app, next: 1, trabajo: make(chan *Item, 10000), ctx: ctx, cancel: cancel}
	// Varias imágenes a la vez; cada conversión además usa varios núcleos (-mt).
	for i := 0; i < max(1, min(4, runtime.NumCPU()/2)); i++ {
		go q.trabajar()
	}
	return q
}

func (q *Cola) Agregar(nombre, formato, entrada, tamano string, bytes int64) int {
	q.mu.Lock()
	it := &Item{ID: q.next, Nombre: nombre, Tamano: tamano, Estado: "cola", Antes: bytes, entrada: entrada, formato: formato}
	q.next++
	q.items = append(q.items, it)
	q.mu.Unlock()
	q.trabajo <- it
	return it.ID
}

func (q *Cola) Snapshot() []Item {
	q.mu.Lock()
	defer q.mu.Unlock()
	out := make([]Item, len(q.items))
	for i, it := range q.items {
		out[i] = *it
	}
	return out
}

func (q *Cola) buscar(id int) *Item {
	q.mu.Lock()
	defer q.mu.Unlock()
	for _, it := range q.items {
		if it.ID == id {
			return it
		}
	}
	return nil
}

// Imagen devuelve el archivo que muestra la miniatura: el WebP si ya está, si no el original.
func (q *Cola) Imagen(id int) string {
	it := q.buscar(id)
	if it == nil {
		return ""
	}
	q.mu.Lock()
	defer q.mu.Unlock()
	if it.Estado == "lista" {
		return it.Archivo
	}
	return it.entrada
}

// ArchivoListo devuelve la ruta del WebP si esa imagen ya terminó.
func (q *Cola) ArchivoListo(id int) string {
	q.mu.Lock()
	defer q.mu.Unlock()
	for _, it := range q.items {
		if it.ID == id && it.Estado == "lista" {
			return it.Archivo
		}
	}
	return ""
}

func (q *Cola) Reintentar(id int) {
	it := q.buscar(id)
	if it == nil {
		return
	}
	q.mu.Lock()
	if it.Estado != "error" || it.entrada == "" {
		q.mu.Unlock()
		return
	}
	it.Estado, it.Error = "cola", nil
	q.mu.Unlock()
	q.trabajo <- it
}

func (q *Cola) LimpiarTerminadas() {
	q.mu.Lock()
	defer q.mu.Unlock()
	quedan := q.items[:0]
	for _, it := range q.items {
		if it.Estado == "lista" || it.Estado == "error" {
			if it.entrada != "" {
				_ = os.Remove(it.entrada)
			}
			continue
		}
		quedan = append(quedan, it)
	}
	q.items = quedan
}

func (q *Cola) Cerrar() { q.cancel() }

func (q *Cola) trabajar() {
	for {
		select {
		case <-q.ctx.Done():
			return
		case it := <-q.trabajo:
			q.convertir(it)
		}
	}
}

func (q *Cola) poner(it *Item, f func()) {
	q.mu.Lock()
	f()
	q.mu.Unlock()
}

func (q *Cola) convertir(it *Item) {
	q.poner(it, func() { it.Estado = "convirtiendo" })
	res, err := q.app.convertirImagen(q.ctx, it.entrada, it.formato, it.Nombre, it.Tamano)
	if err != nil {
		if q.ctx.Err() != nil {
			return
		}
		log.Printf("conversión %q: %v", it.Nombre, err)
		q.poner(it, func() { it.Estado, it.Error = "error", comoMensaje(err) })
		return
	}
	_ = os.Remove(it.entrada)
	q.poner(it, func() {
		it.Estado, it.entrada = "lista", ""
		it.Archivo, it.Salida, it.Despues = res.archivo, filepath.Base(res.archivo), res.bytes
		it.Modo, it.Ancho, it.Alto = res.modo, res.ancho, res.alto
	})
}

type resultado struct {
	archivo     string
	bytes       int64
	modo        string
	ancho, alto int
}

// convertirImagen prepara la imagen (orientación y tamaño) y la pasa al motor WebP.
func (a *App) convertirImagen(ctx context.Context, entrada, formato, nombre, tamano string) (resultado, error) {
	cwebp := a.motor()
	if cwebp == "" {
		return resultado{}, msj("faltaMotor")
	}
	t, ok := tamanos[tamano]
	if !ok {
		t, tamano = tamanos["original"], "original"
	}
	carpeta := a.config().Carpeta
	if err := os.MkdirAll(carpeta, 0o755); err != nil {
		return resultado{}, msj("carpetaNoCreada", "detalle", err.Error())
	}

	orientacion := 1
	if formato == "jpg" {
		orientacion = orientacionJPEG(entrada)
	}
	fuente := entrada
	ancho, alto, err := medidas(entrada, orientacion)
	if err != nil {
		return resultado{}, err
	}
	// El motor lee el archivo tal cual; solo lo preparamos si hay que girarlo o encuadrarlo.
	if t.Ancho == 0 && orientacion == 1 {
		if err := validar(entrada); err != nil {
			return resultado{}, err
		}
	} else {
		img, err := cargar(entrada, orientacion)
		if err != nil {
			return resultado{}, err
		}
		if t.Ancho > 0 {
			img = encajar(img, t.Ancho, t.Alto)
		}
		ancho, alto = img.Rect.Dx(), img.Rect.Dy()
		fuente = strings.TrimSuffix(entrada, filepath.Ext(entrada)) + "-preparada.png"
		if err := guardarPNG(img, fuente); err != nil {
			return resultado{}, msj("noPreparar", "detalle", err.Error())
		}
		defer os.Remove(fuente)
	}

	// El motor escribe junto al destino con un nombre temporal; al terminar se renombra.
	tmp := func(sufijo string) string {
		return filepath.Join(carpeta, fmt.Sprintf(".pngwebp-%s-%s.webp", filepath.Base(entrada), sufijo))
	}

	conPerdida := []string{"-q", calidadFoto, "-m", "6", "-sharp_yuv"}
	sinPerdida := []string{"-lossless", "-z", "6"}
	var elegido, modo string
	if formato == "jpg" {
		elegido, modo = tmp("q"), "calidad-alta"
		if err := a.cwebp(ctx, cwebp, fuente, elegido, conPerdida); err != nil {
			return resultado{}, err
		}
	} else {
		// PNG: las dos codificaciones en paralelo y nos quedamos con la mejor.
		var wg sync.WaitGroup
		var errL, errQ error
		wg.Add(2)
		go func() { defer wg.Done(); errL = a.cwebp(ctx, cwebp, fuente, tmp("l"), sinPerdida) }()
		go func() { defer wg.Done(); errQ = a.cwebp(ctx, cwebp, fuente, tmp("q"), conPerdida) }()
		wg.Wait()
		if errL != nil && errQ != nil {
			return resultado{}, errL
		}
		pesoL, pesoQ := tamanoArchivo(tmp("l")), tamanoArchivo(tmp("q"))
		if errL == nil && (errQ != nil || float64(pesoL) <= float64(pesoQ)*margenSinPerdida) {
			elegido, modo = tmp("l"), "sin-perdida"
			_ = os.Remove(tmp("q"))
			if ancho*alto <= pixelesCompresionMaxima {
				if a.cwebp(ctx, cwebp, fuente, tmp("z"), []string{"-lossless", "-z", "9"}) == nil && tamanoArchivo(tmp("z")) < pesoL {
					_ = os.Remove(tmp("l"))
					elegido = tmp("z")
				} else {
					_ = os.Remove(tmp("z"))
				}
			}
		} else {
			elegido, modo = tmp("q"), "calidad-alta"
			_ = os.Remove(tmp("l"))
		}
	}
	base := strings.TrimSuffix(filepath.Base(nombre), filepath.Ext(nombre))
	destino, err := guardarSinPisar(elegido, carpeta, base+t.Sufijo, ".webp")
	if err != nil {
		_ = os.Remove(elegido)
		return resultado{}, msj("noGuardar", "detalle", err.Error())
	}
	return resultado{archivo: destino, bytes: tamanoArchivo(destino), modo: modo, ancho: ancho, alto: alto}, nil
}

// cwebp ejecuta el motor. Conserva el perfil de color (ICC) y descarta el resto de los
// metadatos (EXIF con la ubicación GPS, datos de la cámara...).
func (a *App) cwebp(ctx context.Context, motor, entrada, salida string, opciones []string) error {
	args := append([]string{"-quiet", "-mt", "-metadata", "icc"}, opciones...)
	args = append(args, entrada, "-o", salida)
	cmd := comando(ctx, motor, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		_ = os.Remove(salida)
		if ctx.Err() != nil {
			return ctx.Err()
		}
		log.Printf("cwebp %v: %v\n%s", args, err, out)
		return msj("motorNoConvirtio")
	}
	return nil
}

var muNombres sync.Mutex

// guardarSinPisar mueve el WebP terminado a un nombre libre. El candado evita que dos
// imágenes con el mismo nombre, convertidas a la vez, terminen en el mismo archivo.
func guardarSinPisar(tmp, carpeta, base, ext string) (string, error) {
	muNombres.Lock()
	defer muNombres.Unlock()
	destino := nombreLibre(carpeta, base, ext)
	return destino, os.Rename(tmp, destino)
}

// nombreLibre evita pisar archivos: «foto.webp», «foto (2).webp», «foto (3).webp»...
func nombreLibre(carpeta, base, ext string) string {
	ruta := filepath.Join(carpeta, base+ext)
	for n := 2; ; n++ {
		if _, err := os.Stat(ruta); errors.Is(err, os.ErrNotExist) {
			return ruta
		}
		ruta = filepath.Join(carpeta, fmt.Sprintf("%s (%d)%s", base, n, ext))
	}
}

func tamanoArchivo(ruta string) int64 {
	st, err := os.Stat(ruta)
	if err != nil {
		return 0
	}
	return st.Size()
}
