# Historial de versiones

Todas las versiones siguen [Versionado Semántico](https://semver.org/lang/es/). Cada una tiene su
etiqueta `vX.Y.Z` y su página en [Releases](../../releases).

## [1.3.2] - 2026-10-22

### Seguridad
- Endurece la API local y limita las dimensiones de imagen, las subidas simultáneas y el tamaño de la cola.

## [1.3.1] - 2026-10-19

### Corregido
- Al instalarla desde Codex no se creaba el acceso directo del escritorio (Codex instala en silencio y la casilla no aparecía). Ahora se crea en la primera instalación; las actualizaciones no lo vuelven a crear si lo borraste.

## [1.3.0] - 2026-10-19

### Añadido
- PngWebp se muestra en el idioma elegido en Codex: español, inglés, francés, alemán, ruso, portugués, chino o japonés. Si Codex está en automático o no está instalado, usa el idioma del PC (si no es ninguno de esos, inglés). Si cambias el idioma en Codex con PngWebp abierta, cambia sola.

## [1.2.1] - 2026-10-19

### Cambiado
- Licencia MIT a nombre de Miguel Bigott: el código es de uso libre manteniendo el aviso de autor. El instalador deja la licencia y las de terceros (`LICENCIAS-TERCEROS.txt`) junto a la app, y el instalador y el .exe dicen «© 2026 Miguel Bigott».

### Corregido
- Las tildes del instalador se veían rotas (por ejemplo «tambiÃ©n»).

## [1.2.0] - 2026-10-19

### Cambiado
- La app se llama **PngWebp**, igual que en Codex: ventana, instalador, ejecutable y textos.
- La carpeta de salida se llama igual que la app: `Imágenes\PngWebp`. Si ya existía como `Imágenes\PNGWebp`, se corrigen sus mayúsculas sin mover nada, y la ruta guardada se actualiza sola.

## [1.1.0] - 2026-10-18

### Cambiado
- Logo nuevo, «Lámina», en el estilo del logo de Codex: una lámina de pergamino con la esquina doblada, un sol de oro, montañas de tinta y el marcapáginas rojo. Trae una versión reforzada para que se lea bien a 16 px.
- Skin «Lámina», hermano de Codex y de Clipvault: el mismo gris neutro, líneas visibles y etiquetas en minúsculas, con `~/pngwebp_` en el encabezado y el azul de la app como acento.
- El ahorro de cada imagen se ve en una etiqueta (−87 %); si el WebP pesa más que el original, la etiqueta lo marca en rojo (+143 %).

### Corregido
- La línea del total decía «imagenes» sin tilde.
- Los nombres con «1x1» se mostraban como «1×1» por la letra; ahora se ven igual que el archivo.

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

[1.2.0]: https://github.com/MiguelBigott/pngwebp/releases/tag/v1.2.0
[1.1.0]: https://github.com/MiguelBigott/pngwebp/releases/tag/v1.1.0
[1.0.0]: https://github.com/MiguelBigott/pngwebp/releases/tag/v1.0.0
