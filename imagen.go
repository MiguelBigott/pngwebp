package main

// Lectura de imágenes, orientación de las fotos y encuadre en las proporciones elegidas.

import (
	"bufio"
	"encoding/binary"
	"image"
	"image/draw"
	"image/jpeg"
	"image/png"
	"io"
	"os"

	xdraw "golang.org/x/image/draw"
)

// Tamano es una salida posible: 0×0 conserva el tamaño original.
type Tamano struct {
	Ancho, Alto int
	Sufijo      string // se agrega al nombre del archivo: «foto (1x1).webp»
}

var tamanos = map[string]Tamano{
	"original": {},
	"1:1":      {1000, 1000, " (1x1)"},
	"16:9":     {1920, 1080, " (16x9)"},
	"9:16":     {1080, 1920, " (9x16)"},
}

// formatoDe reconoce PNG o JPG por sus primeros bytes, no por la extensión.
func formatoDe(cabecera []byte) string {
	switch {
	case len(cabecera) >= 8 && string(cabecera[:8]) == "\x89PNG\r\n\x1a\n":
		return "png"
	case len(cabecera) >= 3 && cabecera[0] == 0xFF && cabecera[1] == 0xD8 && cabecera[2] == 0xFF:
		return "jpg"
	}
	return ""
}

// orientacionJPEG lee la etiqueta EXIF de orientación (1 = derecha; 2-8 = girada o espejada).
// Los celulares guardan la foto «acostada» y anotan aquí cómo mostrarla.
func orientacionJPEG(ruta string) int {
	f, err := os.Open(ruta)
	if err != nil {
		return 1
	}
	defer f.Close()
	r := bufio.NewReader(f)
	var marca [2]byte
	if _, err := io.ReadFull(r, marca[:]); err != nil || marca != [2]byte{0xFF, 0xD8} {
		return 1
	}
	for {
		if _, err := io.ReadFull(r, marca[:]); err != nil || marca[0] != 0xFF {
			return 1
		}
		if marca[1] == 0xDA || marca[1] == 0xD9 { // empiezan los datos de imagen: no hay EXIF
			return 1
		}
		var largo [2]byte
		if _, err := io.ReadFull(r, largo[:]); err != nil {
			return 1
		}
		n := int(binary.BigEndian.Uint16(largo[:])) - 2
		if n < 0 {
			return 1
		}
		datos := make([]byte, n)
		if _, err := io.ReadFull(r, datos); err != nil {
			return 1
		}
		if marca[1] == 0xE1 && len(datos) > 14 && string(datos[:6]) == "Exif\x00\x00" {
			return orientacionTIFF(datos[6:])
		}
	}
}

func orientacionTIFF(t []byte) int {
	if len(t) < 8 {
		return 1
	}
	var orden binary.ByteOrder
	switch string(t[:2]) {
	case "II":
		orden = binary.LittleEndian
	case "MM":
		orden = binary.BigEndian
	default:
		return 1
	}
	ifd := int(orden.Uint32(t[4:8]))
	if ifd+2 > len(t) {
		return 1
	}
	entradas := int(orden.Uint16(t[ifd : ifd+2]))
	for i := 0; i < entradas; i++ {
		p := ifd + 2 + i*12
		if p+12 > len(t) {
			return 1
		}
		if orden.Uint16(t[p:p+2]) == 0x0112 {
			if v := int(orden.Uint16(t[p+8 : p+10])); v >= 1 && v <= 8 {
				return v
			}
			return 1
		}
	}
	return 1
}

// medidas devuelve el ancho y alto con que se ve la imagen (ya girada si hace falta).
func medidas(ruta string, orientacion int) (int, int, error) {
	f, err := os.Open(ruta)
	if err != nil {
		return 0, 0, err
	}
	defer f.Close()
	cfg, _, err := image.DecodeConfig(f)
	if err != nil {
		return 0, 0, msj("noLeerImagen")
	}
	if orientacion >= 5 {
		return cfg.Height, cfg.Width, nil
	}
	return cfg.Width, cfg.Height, nil
}

// validar lee la imagen completa para detectar archivos dañados o cortados, que el motor
// convertiría a medias sin avisar. Si Go no conoce alguna variante poco común del formato,
// se deja pasar: el motor WebP la puede leer igual.
func validar(ruta string) error {
	f, err := os.Open(ruta)
	if err != nil {
		return err
	}
	defer f.Close()
	_, _, err = image.Decode(bufio.NewReader(f))
	switch err.(type) {
	case nil, jpeg.UnsupportedError, png.UnsupportedError:
		return nil
	}
	return msj("imagenDanada")
}

// cargar decodifica la imagen y la endereza según su orientación EXIF.
func cargar(ruta string, orientacion int) (*image.NRGBA, error) {
	f, err := os.Open(ruta)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	img, _, err := image.Decode(bufio.NewReader(f))
	if err != nil {
		return nil, msj("imagenDanada")
	}
	b := img.Bounds()
	base := image.NewNRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	draw.Draw(base, base.Bounds(), img, b.Min, draw.Src)
	return enderezar(base, orientacion), nil
}

// enderezar aplica las 8 orientaciones EXIF (giros de 90° y espejos).
func enderezar(src *image.NRGBA, o int) *image.NRGBA {
	if o <= 1 || o > 8 {
		return src
	}
	w, h := src.Rect.Dx(), src.Rect.Dy()
	dw, dh := w, h
	if o >= 5 {
		dw, dh = h, w
	}
	dst := image.NewNRGBA(image.Rect(0, 0, dw, dh))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			var dx, dy int
			switch o {
			case 2: // espejo horizontal
				dx, dy = w-1-x, y
			case 3: // 180°
				dx, dy = w-1-x, h-1-y
			case 4: // espejo vertical
				dx, dy = x, h-1-y
			case 5: // espejo + 90° antihorario
				dx, dy = y, x
			case 6: // 90° horario
				dx, dy = h-1-y, x
			case 7: // espejo + 90° horario
				dx, dy = h-1-y, w-1-x
			case 8: // 90° antihorario
				dx, dy = y, w-1-x
			}
			copy(dst.Pix[dy*dst.Stride+dx*4:dy*dst.Stride+dx*4+4], src.Pix[y*src.Stride+x*4:y*src.Stride+x*4+4])
		}
	}
	return dst
}

// encajar pone la imagen entera dentro de ancho×alto, centrada y sin deformarla;
// lo que sobra queda transparente.
func encajar(src *image.NRGBA, ancho, alto int) *image.NRGBA {
	w, h := src.Rect.Dx(), src.Rect.Dy()
	escala := min(float64(ancho)/float64(w), float64(alto)/float64(h))
	nw, nh := max(1, int(float64(w)*escala+.5)), max(1, int(float64(h)*escala+.5))
	dst := image.NewNRGBA(image.Rect(0, 0, ancho, alto))
	x0, y0 := (ancho-nw)/2, (alto-nh)/2
	xdraw.CatmullRom.Scale(dst, image.Rect(x0, y0, x0+nw, y0+nh), src, src.Rect, xdraw.Src, nil)
	return dst
}

// guardarPNG escribe un PNG intermedio sin pérdida (rápido: luego lo comprime el motor WebP).
func guardarPNG(img image.Image, ruta string) error {
	f, err := os.Create(ruta)
	if err != nil {
		return err
	}
	enc := png.Encoder{CompressionLevel: png.BestSpeed}
	if err := enc.Encode(f, img); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}
