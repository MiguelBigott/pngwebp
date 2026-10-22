// PngWebp — convierte imágenes PNG y JPG a WebP con la máxima calidad y el menor peso.
// Servidor local + interfaz web embebida en una ventana nativa (WebView2).
package main

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	webview2 "github.com/jchv/go-webview2"
)

const Version = "1.3.2"

// Puerto fijo: sirve también para detectar si la app ya está abierta.
const puerto = "127.0.0.1:47831"

// Tope por imagen: de sobra para fotos de cámara y capturas en 8K.
const maxBytesImagen = 300 << 20

//go:embed web
var webFS embed.FS

type App struct {
	mu        sync.Mutex
	cfg       Config
	cfgPath   string
	dataDir   string
	entradas  string
	subidas   chan struct{}
	cola      *Cola
	verMotor  string
	verMotorU sync.Once
}

type Config struct {
	Carpeta string `json:"carpeta"`
	Tamano  string `json:"tamano"`
}

func main() {
	dataDir := carpetaDatos()
	corregirMayusculas(dataDir)
	_ = os.MkdirAll(dataDir, 0o755)
	logFile, err := os.OpenFile(filepath.Join(dataDir, "pngwebp.log"), os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err == nil {
		log.SetOutput(logFile)
	}

	ln, err := net.Listen("tcp", puerto)
	if err != nil {
		// Ya hay una instancia abierta: traemos su ventana al frente.
		traerAlFrente()
		return
	}

	app := &App{dataDir: dataDir, cfgPath: filepath.Join(dataDir, "config.json"), entradas: filepath.Join(dataDir, "entradas"), subidas: make(chan struct{}, 2)}
	// Copias temporales de las imágenes en curso: lo que quedó de una sesión anterior sobra.
	_ = os.RemoveAll(app.entradas)
	_ = os.MkdirAll(app.entradas, 0o755)
	app.cargarConfig()
	app.cola = NuevaCola(app)

	mux := http.NewServeMux()
	sub, _ := fs.Sub(webFS, "web")
	mux.Handle("/", http.FileServer(http.FS(sub)))
	app.rutas(mux)

	go func() {
		server := &http.Server{Handler: seguridadHTTP(mux), ReadHeaderTimeout: 5 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 1 << 20}
		if err := server.Serve(ln); err != nil {
			log.Println("servidor:", err)
		}
	}()

	url := "http://" + puerto + "/"
	if os.Getenv("PNGWEBP_SIN_VENTANA") != "" {
		log.Println("modo prueba, escuchando en", url)
		select {}
	}

	ancho, alto := tamanoInicial()
	w := webview2.NewWithOptions(webview2.WebViewOptions{
		AutoFocus: true,
		DataPath:  filepath.Join(dataDir, "webview"),
		WindowOptions: webview2.WindowOptions{
			Title: tituloVentana, Width: ancho, Height: alto, Center: true, IconId: idIcono,
		},
	})
	if w == nil {
		mensaje(textosDe(idiomaApp()).FaltaWebView)
		return
	}
	defer w.Destroy()
	prepararVentana(uintptr(w.Window()))
	w.SetSize(900, 620, webview2.HintMin)
	_ = w.Bind("caArrastrar", ventanaArrastrar)
	_ = w.Bind("caRedimensionar", ventanaRedimensionar)
	_ = w.Bind("caMinimizar", ventanaMinimizar)
	_ = w.Bind("caMaximizar", ventanaMaximizar)
	_ = w.Bind("caMaximizada", ventanaMaximizada)
	_ = w.Bind("caCerrar", ventanaCerrar)
	w.Navigate(url)
	w.Run()

	app.cola.Cerrar()
	time.Sleep(300 * time.Millisecond)
	_ = os.RemoveAll(app.entradas)
}

func (a *App) cargarConfig() {
	a.cfg = Config{Carpeta: carpetaSalidaPorDefecto(), Tamano: "original"}
	b, err := os.ReadFile(a.cfgPath)
	if err == nil {
		_ = json.Unmarshal(b, &a.cfg)
	}
	if a.cfg.Carpeta == "" {
		a.cfg.Carpeta = carpetaSalidaPorDefecto()
	}
	// Desde la 1.2.0 la carpeta se llama como la app («PngWebp»): se corrige la ruta guardada
	// y las mayúsculas de la carpeta si ya existía.
	if def := carpetaSalidaPorDefecto(); strings.EqualFold(a.cfg.Carpeta, def) {
		a.cfg.Carpeta = def
		corregirMayusculas(def)
	}
	if _, ok := tamanos[a.cfg.Tamano]; !ok {
		a.cfg.Tamano = "original"
	}
}

func (a *App) guardarConfig() {
	b, _ := json.MarshalIndent(a.cfg, "", "  ")
	_ = os.WriteFile(a.cfgPath, b, 0o644)
}

func (a *App) config() Config {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.cfg
}

// motor devuelve la ruta de cwebp.exe, que viene en la carpeta «componentes» junto al .exe.
func (a *App) motor() string {
	exe, err := os.Executable()
	if err != nil {
		return ""
	}
	ruta := filepath.Join(filepath.Dir(exe), "componentes", "cwebp.exe")
	if _, err := os.Stat(ruta); err != nil {
		return ""
	}
	return ruta
}

// versionMotor pregunta una sola vez la versión de cwebp («1.6.0»).
func (a *App) versionMotor() string {
	a.verMotorU.Do(func() {
		if m := a.motor(); m != "" {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if out, err := comando(ctx, m, "-version").Output(); err == nil {
				a.verMotor = strings.TrimSpace(strings.SplitN(string(out), "\n", 2)[0])
			}
		}
	})
	return a.verMotor
}

// ---------------- API ----------------

func responder(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(w).Encode(v)
}

// fallar responde con un error que la interfaz muestra en su idioma.
func fallar(w http.ResponseWriter, status int, m *Mensaje) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]*Mensaje{"error": m})
}

func leerJSON(r *http.Request, v any) error {
	if r.Method != http.MethodPost {
		return msj("peticionInvalida")
	}
	if err := json.NewDecoder(http.MaxBytesReader(nil, r.Body, 1<<20)).Decode(v); err != nil {
		return msj("peticionInvalida")
	}
	return nil
}

// Solo acepta la interfaz local y exige Origin exacto para las operaciones no seguras.
func soloLocal(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Host != puerto {
			fallar(w, http.StatusForbidden, msj("accesoDenegado"))
			return
		}
		origen := r.Header.Get("Origin")
		seguro := r.Method == http.MethodGet || r.Method == http.MethodHead
		if (!seguro && origen != "http://"+puerto) || (seguro && origen != "" && origen != "http://"+puerto) {
			fallar(w, http.StatusForbidden, msj("accesoDenegado"))
			return
		}
		h(w, r)
	}
}

func seguridadHTTP(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; img-src 'self' data: blob:; style-src 'self' 'unsafe-inline'; font-src 'self' data:; connect-src 'self'; worker-src 'self' blob:; object-src 'none'; base-uri 'none'; frame-ancestors 'none'")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("X-Frame-Options", "DENY")
		h.ServeHTTP(w, r)
	})
}

func (a *App) rutas(mux *http.ServeMux) {
	mux.HandleFunc("/api/estado", soloLocal(func(w http.ResponseWriter, r *http.Request) {
		responder(w, map[string]any{
			"version": Version,
			"idioma":  idiomaApp(),
			"config":  a.config(),
			"motor":   map[string]any{"listo": a.motor() != "", "version": a.versionMotor()},
			"cola":    a.cola.Snapshot(),
		})
	}))

	// agregar recibe una imagen (el archivo tal cual en el cuerpo) y la pone en la cola.
	mux.HandleFunc("/api/agregar", soloLocal(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			fallar(w, http.StatusMethodNotAllowed, msj("peticionInvalida"))
			return
		}
		select {
		case a.subidas <- struct{}{}:
			defer func() { <-a.subidas }()
		default:
			fallar(w, http.StatusTooManyRequests, msj("colaLlena"))
			return
		}
		nombre := filepath.Base(strings.ReplaceAll(r.URL.Query().Get("nombre"), "\\", "/"))
		if nombre == "" || nombre == "." || nombre == "/" {
			nombre = "imagen"
		}
		tamano := r.URL.Query().Get("tamano")
		if _, ok := tamanos[tamano]; !ok {
			tamano = "original"
		}
		cuerpo := http.MaxBytesReader(w, r.Body, maxBytesImagen)
		cabecera := make([]byte, 8)
		n, _ := io.ReadFull(cuerpo, cabecera)
		formato := formatoDe(cabecera[:n])
		if formato == "" {
			fallar(w, http.StatusUnsupportedMediaType, msj("noPngJpg", "nombre", nombre))
			return
		}
		ruta := filepath.Join(a.entradas, fmt.Sprintf("%d-%d.%s", time.Now().UnixNano(), os.Getpid(), formato))
		f, err := os.Create(ruta)
		if err != nil {
			fallar(w, http.StatusInternalServerError, msj("noRecibir"))
			return
		}
		_, _ = f.Write(cabecera[:n])
		copiados, err := io.Copy(f, cuerpo)
		f.Close()
		if err != nil {
			_ = os.Remove(ruta)
			fallar(w, http.StatusRequestEntityTooLarge, msj("demasiadoGrande", "nombre", nombre))
			return
		}
		id, err := a.cola.Agregar(nombre, formato, ruta, tamano, copiados+int64(n))
		if err != nil {
			_ = os.Remove(ruta)
			fallar(w, http.StatusTooManyRequests, msj("colaLlena"))
			return
		}
		responder(w, map[string]int{"id": id})
	}))

	mux.HandleFunc("/api/tamano", soloLocal(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Tamano string `json:"tamano"`
		}
		if err := leerJSON(r, &req); err != nil {
			fallar(w, http.StatusBadRequest, comoMensaje(err))
			return
		}
		if _, ok := tamanos[req.Tamano]; !ok {
			fallar(w, http.StatusBadRequest, msj("tamanoDesconocido"))
			return
		}
		a.mu.Lock()
		a.cfg.Tamano = req.Tamano
		a.guardarConfig()
		a.mu.Unlock()
		responder(w, map[string]bool{"ok": true})
	}))

	// imagen sirve la miniatura de un elemento de la cola.
	mux.HandleFunc("/api/imagen", soloLocal(func(w http.ResponseWriter, r *http.Request) {
		id, _ := strconv.Atoi(r.URL.Query().Get("id"))
		ruta := a.cola.Imagen(id)
		if ruta == "" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		http.ServeFile(w, r, ruta)
	}))

	mux.HandleFunc("/api/reintentar", soloLocal(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			ID int `json:"id"`
		}
		if err := leerJSON(r, &req); err != nil {
			fallar(w, http.StatusBadRequest, comoMensaje(err))
			return
		}
		a.cola.Reintentar(req.ID)
		responder(w, map[string]bool{"ok": true})
	}))

	mux.HandleFunc("/api/limpiar", soloLocal(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			fallar(w, http.StatusMethodNotAllowed, msj("peticionInvalida"))
			return
		}
		a.cola.LimpiarTerminadas()
		responder(w, map[string]bool{"ok": true})
	}))

	mux.HandleFunc("/api/carpeta/elegir", soloLocal(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			fallar(w, http.StatusMethodNotAllowed, msj("peticionInvalida"))
			return
		}
		actual := a.config().Carpeta
		elegida, err := elegirCarpeta(textosDe(idiomaApp()).ElegirCarpeta, actual)
		if err != nil || elegida == "" {
			responder(w, map[string]any{"carpeta": actual, "cambiada": false})
			return
		}
		a.mu.Lock()
		a.cfg.Carpeta = elegida
		a.guardarConfig()
		a.mu.Unlock()
		responder(w, map[string]any{"carpeta": elegida, "cambiada": true})
	}))

	mux.HandleFunc("/api/carpeta/abrir", soloLocal(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			ID int `json:"id"`
		}
		_ = leerJSON(r, &req)
		if archivo := a.cola.ArchivoListo(req.ID); archivo != "" {
			mostrarArchivo(archivo)
			responder(w, map[string]bool{"ok": true})
			return
		}
		c := a.config().Carpeta
		_ = os.MkdirAll(c, 0o755)
		abrirCarpeta(c)
		responder(w, map[string]bool{"ok": true})
	}))
}
