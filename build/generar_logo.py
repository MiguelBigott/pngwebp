"""Logo de PngWebp «Lámina», en el estilo del códice de Codex: una lámina de pergamino con la
esquina doblada, un sol de oro, montañas de tinta y el marcapáginas rojo de la rúbrica.

    python build/generar_logo.py

Escribe:
  web/logo.svg       logo para el encabezado de la app
  build/logo.png     logo para el README (256 px)
  build/app.ico      icono del .exe: versión reforzada en 16-32 px, normal desde 36 px
  build/lateral.bmp  imagen lateral del instalador (al doble de tamaño, para pantallas con escala)
Requiere Pillow y Microsoft Edge (renderiza el SVG).
"""
import os
import subprocess
import tempfile

from PIL import Image

BUILD = os.path.dirname(os.path.abspath(__file__))
RAIZ = os.path.dirname(BUILD)
EDGE = r"C:\Program Files (x86)\Microsoft\Edge\Application\msedge.exe"

TINTA, BORDE, PERGAMINO, ORO, RUBRICA = "#16120e", "#3a3128", "#ece2cf", "#c9a45c", "#c8483a"
LAPIS, LAPIS_CLARO = "#2c4a8c", "#6f93df"


def svg(chico=False):
    """Logo en viewBox 512. `chico`: lámina más grande y detalles más gruesos para 16-32 px."""
    if chico:
        hoja = '<path d="M106 62 H316 L396 142 V416 H106 Z" fill="{p}"/>'
        esquina = '<path d="M316 62 V142 H396" fill="{t}" stroke="{t}" stroke-width="16" stroke-linejoin="round"/>'
        sol = '<circle cx="306" cy="214" r="42" fill="{o}"/>'
        montes = '<path d="M132 376 L210 278 L262 334 L304 296 L370 376 Z" fill="{t}"/>'
        marca = '<path d="M290 416 V478 L306 462 L322 478 V416 Z" fill="{r}"/>'
        borde = 0
    else:
        hoja = '<path d="M146 92 H310 L366 148 V392 H146 Z" fill="{p}"/>'
        esquina = '<path d="M310 92 V148 H366" fill="{t}" stroke="{t}" stroke-width="10" stroke-linejoin="round"/>'
        sol = '<circle cx="300" cy="216" r="28" fill="{o}"/>'
        montes = '<path d="M172 356 L222 288 L256 324 L288 296 L340 356 Z" fill="{t}"/>'
        marca = '<path d="M296 392 V448 L306 438 L316 448 V392 Z" fill="{r}"/>'
        borde = 10
    partes = "\n  ".join(x.format(p=PERGAMINO, t=TINTA, o=ORO, r=RUBRICA) for x in (hoja, esquina, sol, montes, marca))
    return f'''<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 512 512" width="512" height="512">
  <rect x="8" y="8" width="496" height="496" rx="112" fill="{TINTA}" stroke="{BORDE}" stroke-width="{borde}"/>
  {partes}
</svg>
'''


def renderizar(contenido_html, ancho, alto, destino):
    with tempfile.TemporaryDirectory() as perfil:
        html = os.path.join(perfil, "p.html")
        with open(html, "w", encoding="utf-8") as f:
            f.write(contenido_html)
        subprocess.run([EDGE, "--headless", "--disable-gpu", "--hide-scrollbars", "--allow-file-access-from-files",
                        f"--user-data-dir={perfil}", "--default-background-color=00000000",
                        f"--window-size={ancho},{alto}", "--virtual-time-budget=3000",
                        f"--screenshot={destino}", "file:///" + html.replace(os.sep, "/")],
                       check=True, capture_output=True, timeout=60)


def png_de_svg(texto_svg, lado, destino):
    with tempfile.TemporaryDirectory() as tmp:
        ruta = os.path.join(tmp, "logo.svg")
        with open(ruta, "w", encoding="utf-8") as f:
            f.write(texto_svg)
        renderizar(f'<html><body style="margin:0;background:transparent"><img src="file:///{ruta.replace(os.sep, "/")}" '
                   f'width="{lado}" height="{lado}" style="display:block"></body></html>', lado, lado, destino)
    im = Image.open(destino).convert("RGBA")
    im.load()
    return im


def lateral(logo_png, destino):
    fuente = os.path.join(RAIZ, "web", "fonts", "inter-latin-600-normal.woff2").replace(os.sep, "/")
    with tempfile.TemporaryDirectory() as tmp:
        png = os.path.join(tmp, "lateral.png")
        renderizar(f'''<html><head><style>
@font-face {{ font-family: Titulo; src: url("file:///{fuente}"); font-weight: 600; }}
html, body {{ margin: 0; width: 328px; height: 628px; overflow: hidden; background: {TINTA}; }}
body {{ display: flex; flex-direction: column; align-items: center; color: {PERGAMINO}; }}
img {{ width: 200px; height: 200px; margin-top: 150px; }}
h1 {{ margin: 30px 0 0; font-family: Titulo; font-weight: 600; font-size: 46px; letter-spacing: -.03em; }}
h1 span {{ color: {LAPIS_CLARO}; }}
</style></head><body><img src="file:///{logo_png.replace(os.sep, "/")}"><h1>Png<span>Webp</span></h1></body></html>''', 328, 628, png)
        Image.open(png).convert("RGB").save(destino, format="BMP")


def main():
    with open(os.path.join(RAIZ, "web", "logo.svg"), "w", encoding="utf-8", newline="\n") as f:
        f.write(svg())
    with tempfile.TemporaryDirectory() as tmp:
        normal = png_de_svg(svg(), 1024, os.path.join(tmp, "normal.png"))
        chico = png_de_svg(svg(chico=True), 1024, os.path.join(tmp, "chico.png"))
        normal.resize((256, 256), Image.LANCZOS).save(os.path.join(BUILD, "logo.png"))
        tam = [16, 20, 24, 30, 32, 36, 40, 48, 64, 128, 256]
        cuadros = [(chico if s <= 32 else normal).resize((s, s), Image.LANCZOS) for s in tam]
        cuadros[-1].save(os.path.join(BUILD, "app.ico"), format="ICO",
                         sizes=[(s, s) for s in tam], append_images=cuadros[:-1])
        logo_lat = os.path.join(tmp, "lat.png")
        normal.resize((400, 400), Image.LANCZOS).save(logo_lat)
        lateral(logo_lat, os.path.join(BUILD, "lateral.bmp"))
    print("logo generado")


if __name__ == "__main__":
    main()
