"use strict";

const $ = (s) => document.querySelector(s);

async function api(ruta, datos) {
  const opciones = { headers: { "X-PNGWebp": "1" } };
  if (datos !== undefined) {
    opciones.method = "POST";
    opciones.headers["Content-Type"] = "application/json";
    opciones.body = JSON.stringify(datos);
  }
  const r = await fetch(ruta, opciones);
  const json = await r.json().catch(() => ({}));
  if (!r.ok) throw new Error(json.error || "Error inesperado");
  return json;
}

let aviso_t;
function avisar(texto, esError) {
  const el = $("#aviso");
  el.textContent = texto;
  el.classList.toggle("error", !!esError);
  el.hidden = false;
  clearTimeout(aviso_t);
  aviso_t = setTimeout(() => (el.hidden = true), esError ? 7000 : 4000);
}

function peso(b) {
  const num = (n, d) => n.toLocaleString("es", { minimumFractionDigits: d, maximumFractionDigits: d });
  if (b < 1024) return `${b} B`;
  if (b < 1024 * 1024) return `${num(b / 1024, b < 100 * 1024 ? 1 : 0)} KB`;
  return `${num(b / (1024 * 1024), 1)} MB`;
}

/* ---------------- Recibir imágenes ---------------- */

const esImagen = (nombre) => /\.(png|jpe?g)$/i.test(nombre);
const tamanoElegido = () => document.querySelector('input[name="tamano"]:checked')?.value || "original";

// subir manda cada imagen al servidor, una tras otra, con el tamaño elegido en ese momento.
let subiendo = Promise.resolve();
function subir(archivos) {
  const imagenes = archivos.filter((f) => esImagen(f.name));
  const omitidos = archivos.length - imagenes.length;
  if (!imagenes.length) {
    if (omitidos) avisar("Solo se convierten imágenes PNG o JPG.", true);
    return;
  }
  const tamano = tamanoElegido();
  subiendo = subiendo.then(async () => {
    let bien = 0;
    const fallas = [];
    for (const f of imagenes) {
      try {
        const r = await fetch(`/api/agregar?nombre=${encodeURIComponent(f.name)}&tamano=${encodeURIComponent(tamano)}`,
          { method: "POST", headers: { "X-PNGWebp": "1" }, body: f });
        if (!r.ok) throw new Error((await r.json().catch(() => ({}))).error || f.name);
        bien++;
        refrescar();
      } catch (e) {
        fallas.push(e.message);
      }
    }
    const partes = [];
    if (bien) partes.push(bien === 1 ? "1 imagen en camino." : `${bien} imágenes en camino.`);
    if (omitidos) partes.push(omitidos === 1 ? "Se omitió 1 archivo que no es PNG ni JPG." : `Se omitieron ${omitidos} archivos que no son PNG ni JPG.`);
    if (fallas.length) partes.push(fallas[0] + (fallas.length > 1 ? ` (y ${fallas.length - 1} más)` : ""));
    avisar(partes.join(" "), fallas.length > 0 && !bien);
  });
}

// Lee una carpeta soltada, con sus subcarpetas.
async function archivosDeEntrada(entrada) {
  if (entrada.isFile) return [await new Promise((ok, mal) => entrada.file(ok, mal))];
  if (!entrada.isDirectory) return [];
  const lector = entrada.createReader();
  const hijos = [];
  for (;;) {
    const tanda = await new Promise((ok, mal) => lector.readEntries(ok, mal));
    if (!tanda.length) break;
    hijos.push(...tanda);
  }
  const listas = await Promise.all(hijos.map(archivosDeEntrada));
  return listas.flat();
}

// Arrastrar y soltar sobre toda la ventana.
let profundidad = 0;
const conArchivos = (ev) => [...(ev.dataTransfer?.types || [])].includes("Files");
window.addEventListener("dragenter", (ev) => {
  if (!conArchivos(ev)) return;
  ev.preventDefault();
  if (++profundidad === 1) { $("#arrastre").hidden = false; $("#soltar").classList.add("encima"); }
});
window.addEventListener("dragover", (ev) => {
  if (!conArchivos(ev)) return;
  ev.preventDefault();
  ev.dataTransfer.dropEffect = "copy";
});
window.addEventListener("dragleave", (ev) => {
  if (!conArchivos(ev)) return;
  if (--profundidad <= 0) { profundidad = 0; $("#arrastre").hidden = true; $("#soltar").classList.remove("encima"); }
});
window.addEventListener("drop", async (ev) => {
  if (!conArchivos(ev)) return;
  ev.preventDefault();
  profundidad = 0;
  $("#arrastre").hidden = true;
  $("#soltar").classList.remove("encima");
  const entradas = [...ev.dataTransfer.items].map((i) => i.webkitGetAsEntry?.()).filter(Boolean);
  const archivos = entradas.length ? (await Promise.all(entradas.map(archivosDeEntrada))).flat() : [...ev.dataTransfer.files];
  subir(archivos);
});

$("#btn-archivos").addEventListener("click", () => $("#in-archivos").click());
$("#btn-carpeta").addEventListener("click", () => $("#in-carpeta").click());
for (const id of ["#in-archivos", "#in-carpeta"]) {
  $(id).addEventListener("change", (ev) => {
    subir([...ev.target.files]);
    ev.target.value = "";
  });
}

document.querySelectorAll('input[name="tamano"]').forEach((r) => r.addEventListener("change", () => {
  api("/api/tamano", { tamano: r.value }).catch((e) => avisar(e.message, true));
}));

/* ---------------- Las iluminadas ---------------- */

const MODO = { "sin-perdida": "sin pérdida", "calidad-alta": "calidad alta" };

function detalle(it) {
  switch (it.estado) {
    case "cola": return [`${peso(it.antes)} · en espera`];
    case "convirtiendo": return [`${peso(it.antes)} · convirtiendo…`];
    case "error": return [it.error || "No se pudo convertir"];
    case "lista": {
      const dif = Math.round((1 - it.despues / it.antes) * 100);
      const ahorro = dif > 0 ? `${dif} % menos` : null;
      if (ahorro) return [`${peso(it.antes)} → ${peso(it.despues)} · ${ahorro}`];
      return [`${peso(it.antes)} → ${peso(it.despues)} · `, { mas: `pesa ${-dif} % más` }];
    }
  }
  return [""];
}

function boton(texto, fn) {
  const b = document.createElement("button");
  b.type = "button";
  b.className = "btn";
  b.textContent = texto;
  b.addEventListener("click", fn);
  return b;
}

const ESTADO = { cola: "en cola", convirtiendo: "convirtiendo", lista: "iluminada", error: "" };
const filas = new Map(); // id -> li

function pintarCola(cola) {
  const lista = $("#lista");
  const vivos = new Set();
  // Lo más reciente arriba.
  [...cola].reverse().forEach((it, i) => {
    vivos.add(it.id);
    let li = filas.get(it.id);
    if (!li) {
      li = $("#tpl-item").content.firstElementChild.cloneNode(true);
      filas.set(it.id, li);
    }
    if (lista.children[i] !== li) lista.insertBefore(li, lista.children[i] || null);

    li.className = `item ${it.estado}`;
    const img = li.querySelector(".mini img");
    const src = `/api/imagen?id=${it.id}&e=${it.estado === "lista" ? "webp" : "original"}`;
    if (img.getAttribute("src") !== src) img.src = src;
    const nombre = it.estado === "lista" ? it.salida : it.nombre;
    li.querySelector(".nombre").textContent = nombre;
    li.querySelector(".nombre").title = it.archivo || it.nombre;
    li.querySelector(".detalle").replaceChildren(...detalle(it).map((p) => {
      if (typeof p === "string") return p;
      const s = document.createElement("span");
      s.className = "mas";
      s.textContent = p.mas;
      return s;
    }));
    li.querySelector(".detalle2").textContent = it.estado === "lista" ? `${MODO[it.modo] || ""} · ${it.ancho} × ${it.alto}` : "";
    li.querySelector(".est").textContent = ESTADO[it.estado] || "";

    const acc = li.querySelector(".acciones");
    if (acc.dataset.firma !== it.estado) {
      acc.dataset.firma = it.estado;
      acc.replaceChildren();
      if (it.estado === "lista") acc.append(boton("Mostrar", () => api("/api/carpeta/abrir", { id: it.id })));
      if (it.estado === "error") acc.append(boton("Reintentar", () => api("/api/reintentar", { id: it.id }).then(refrescar)));
    }
  });
  for (const [id, li] of filas) {
    if (!vivos.has(id)) { li.remove(); filas.delete(id); }
  }
  $("#vacio").hidden = cola.length > 0;
  $("#btn-limpiar").hidden = !cola.some((it) => it.estado === "lista" || it.estado === "error");

  const listas = cola.filter((it) => it.estado === "lista");
  const antes = listas.reduce((s, it) => s + it.antes, 0);
  const despues = listas.reduce((s, it) => s + it.despues, 0);
  const total = $("#total");
  total.hidden = !listas.length || despues >= antes;
  if (!total.hidden) {
    const n = listas.length;
    total.replaceChildren("Ahorraste ", Object.assign(document.createElement("b"), { textContent: peso(antes - despues) }),
      ` en ${n} imagen${n === 1 ? "" : "es"}: ${n === 1 ? "pesa" : "pesan"} un ${Math.round((1 - despues / antes) * 100)} % menos.`);
  }
  const pendientes = cola.filter((it) => it.estado === "cola" || it.estado === "convirtiendo").length;
  document.title = pendientes ? `PNGWebp (${pendientes})` : "PNGWebp";
}

$("#btn-limpiar").addEventListener("click", () => api("/api/limpiar", {}).then(refrescar));

/* ---------------- Carpeta y motor ---------------- */

function pintarConfig(cfg, primera) {
  const r = $("#ruta-destino");
  r.textContent = cfg.carpeta;
  r.title = cfg.carpeta;
  if (primera) {
    const radio = document.querySelector(`input[name="tamano"][value="${cfg.tamano}"]`);
    (radio || document.querySelector('input[name="tamano"][value="original"]')).checked = true;
  }
}

function pintarMotor(m) {
  const v = $("#ver-motor");
  v.textContent = m.listo ? (m.version || "instalado") : "no encontrado";
  v.classList.toggle("falta", !m.listo);
  $("#motor-falta").hidden = m.listo;
}

$("#btn-cambiar-carpeta").addEventListener("click", async (e) => {
  e.currentTarget.disabled = true;
  try {
    const r = await api("/api/carpeta/elegir", {});
    $("#ruta-destino").textContent = r.carpeta;
    if (r.cambiada) avisar("Las imágenes WebP se guardarán en la nueva carpeta.");
  } finally {
    e.currentTarget.disabled = false;
  }
});
$("#btn-abrir-carpeta").addEventListener("click", () => api("/api/carpeta/abrir", {}));

/* ---------------- Ciclo ---------------- */

let primera = true;
async function refrescar() {
  try {
    const e = await api("/api/estado");
    pintarConfig(e.config, primera);
    pintarMotor(e.motor);
    pintarCola(e.cola);
    if (primera) $("#version").textContent = `PNGWebp ${e.version} · de la biblioteca Codex`;
    primera = false;
  } catch {
    /* el servidor puede estar cerrándose */
  }
}

refrescar();
setInterval(refrescar, 700);

/* ---------------- Ventana (solo dentro de la app, no en un navegador) ---------------- */

if (typeof window.caArrastrar === "function") {
  $("#controles").hidden = false;
  const barra = $("#barra");
  const interactivo = "button, a, input, select, textarea";
  barra.addEventListener("mousedown", (ev) => {
    if (ev.button !== 0 || ev.detail > 1 || ev.target.closest(interactivo)) return;
    window.caArrastrar();
  });
  const actualizarMaximizada = async () => {
    const max = await window.caMaximizada();
    document.body.classList.toggle("maximizada", max);
    $("#btn-max").setAttribute("aria-label", max ? "Restaurar" : "Maximizar");
  };
  barra.addEventListener("dblclick", (ev) => {
    if (!ev.target.closest(interactivo)) window.caMaximizar().then(actualizarMaximizada);
  });
  document.querySelectorAll(".redim").forEach((r) => r.addEventListener("mousedown", (ev) => {
    if (ev.button === 0) window.caRedimensionar(r.dataset.borde);
  }));
  $("#btn-min").addEventListener("click", () => window.caMinimizar());
  $("#btn-max").addEventListener("click", () => window.caMaximizar().then(actualizarMaximizada));
  $("#btn-cerrar").addEventListener("click", () => window.caCerrar());
  window.addEventListener("resize", actualizarMaximizada);
  actualizarMaximizada();
} else {
  document.querySelectorAll(".redim").forEach((r) => r.remove());
}
