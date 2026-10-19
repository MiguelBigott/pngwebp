//go:build windows

package main

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"unsafe"
)

const crearSinVentana = 0x08000000 // CREATE_NO_WINDOW

func ocultar(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: crearSinVentana}
}

// comando crea un proceso sin ventana de consola.
func comando(ctx context.Context, nombre string, args ...string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, nombre, args...)
	ocultar(cmd)
	return cmd
}

func carpetaDatos() string {
	if d := os.Getenv("LOCALAPPDATA"); d != "" {
		return filepath.Join(d, "PngWebp")
	}
	d, _ := os.UserConfigDir()
	return filepath.Join(d, "PngWebp")
}

// carpetaSalidaPorDefecto devuelve "PngWebp" dentro de la carpeta Imágenes
// del usuario (respeta si la movió a otro disco).
func carpetaSalidaPorDefecto() string {
	if v := carpetaImagenes(); v != "" {
		return filepath.Join(v, "PngWebp")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "Pictures", "PngWebp")
}

// FOLDERID_Pictures {33E28130-4E1E-4676-835A-98395C3BC3BB}
var folderIDImagenes = syscall.GUID{Data1: 0x33E28130, Data2: 0x4E1E, Data3: 0x4676,
	Data4: [8]byte{0x83, 0x5A, 0x98, 0x39, 0x5C, 0x3B, 0xC3, 0xBB}}

func carpetaImagenes() string {
	var p *uint16
	r, _, _ := syscall.NewLazyDLL("shell32.dll").NewProc("SHGetKnownFolderPath").Call(
		uintptr(unsafe.Pointer(&folderIDImagenes)), 0, 0, uintptr(unsafe.Pointer(&p)))
	if p == nil {
		return ""
	}
	defer syscall.NewLazyDLL("ole32.dll").NewProc("CoTaskMemFree").Call(uintptr(unsafe.Pointer(p)))
	if r != 0 {
		return ""
	}
	return syscall.UTF16ToString(unsafe.Slice(p, 32768))
}

func powershell(script string) (string, error) {
	cmd := exec.Command("powershell", "-NoProfile", "-STA", "-ExecutionPolicy", "Bypass", "-Command", script)
	ocultar(cmd)
	out, err := cmd.Output()
	return strings.TrimSpace(string(out)), err
}

func comillasPS(s string) string { return "'" + strings.ReplaceAll(s, "'", "''") + "'" }

func elegirCarpeta(titulo, inicial string) (string, error) {
	script := `[Console]::OutputEncoding=[Text.Encoding]::UTF8
Add-Type -AssemblyName System.Windows.Forms
[System.Windows.Forms.Application]::EnableVisualStyles()
$f = New-Object System.Windows.Forms.Form -Property @{TopMost=$true; ShowInTaskbar=$false; Width=0; Height=0; StartPosition='CenterScreen'}
$d = New-Object System.Windows.Forms.FolderBrowserDialog
$d.Description = ` + comillasPS(titulo) + `
$d.ShowNewFolderButton = $true
$d.SelectedPath = ` + comillasPS(inicial) + `
if ($d.ShowDialog($f) -eq 'OK') { $d.SelectedPath }`
	return powershell(script)
}

func abrirCarpeta(ruta string) {
	_ = exec.Command("explorer.exe", ruta).Start()
}

func mostrarArchivo(ruta string) {
	c := exec.Command("explorer.exe")
	c.SysProcAttr = &syscall.SysProcAttr{CmdLine: `explorer.exe /select,"` + ruta + `"`}
	_ = c.Start()
}
