# Historial de versiones

Todas las versiones siguen [Versionado Semántico](https://semver.org/lang/es/). Cada una tiene su
etiqueta `vX.Y.Z` y su página en [Releases](../../releases).

## [1.0.0] - 2026-10-18

Primera versión como app de Windows. Antes era un conversor en HTML que dejaba todas las imágenes
en 1000 × 1000 con calidad 0,85.

### Añadido
- Convierte PNG y JPG a WebP con el motor oficial de Google (libwebp 1.6.0), incluido en el instalador.
- Máxima calidad con el menor peso, sin controles: los JPG salen con calidad 92; los PNG se prueban sin pérdida y con pérdida, y se guarda el que conviene (capturas y logos idénticos píxel a píxel; fotos mucho más livianas).
- Tamaños: original, 1:1 (1000 × 1000), 16:9 (1920 × 1080) y 9:16 (1080 × 1920), sin deformar la imagen.
- Arrastrar imágenes o carpetas enteras a la ventana, o elegirlas desde el PC.
- Respeta la orientación de las fotos del celular.
- Conserva el perfil de color y descarta los demás metadatos (como la ubicación GPS).
- Avisa si una imagen está dañada o no es un PNG o JPG.
- Skin «Iluminador» y logo de la miniatura iluminada, hermanos de Clipvault.
- Ventana nativa de Windows (WebView2) con icono nítido en la barra de tareas.
- Forma parte de la suite [Codex](https://github.com/MiguelBigott/codex).

[1.0.0]: https://github.com/MiguelBigott/pngwebp/releases/tag/v1.0.0
