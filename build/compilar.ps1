# Compila PNGWebp.exe y el instalador en dist\.
# Requiere: Go, NSIS (makensis) y go-winres (go install github.com/tc-hib/go-winres@latest).
#
# El motor WebP (cwebp.exe) sale de la compilación oficial de Google para Windows, se verifica
# con SHA-256 y se guarda en build\componentes (fuera de git).
param()

$ErrorActionPreference = 'Stop'
$ProgressPreference = 'SilentlyContinue'
$raiz = Split-Path $PSScriptRoot -Parent
Set-Location $raiz

$version = (Select-String -Path main.go -Pattern 'const Version = "(.+)"').Matches[0].Groups[1].Value
$goExe = (Get-Command go -ErrorAction SilentlyContinue).Source
if (-not $goExe) { $goExe = 'C:\Program Files\Go\bin\go.exe' }
$winres = Join-Path (& $goExe env GOPATH) 'bin\go-winres.exe'
$makensis = 'C:\Program Files (x86)\NSIS\makensis.exe'

Write-Host "PNGWebp $version"

# ---------- Motor WebP ----------
# libwebp 1.6.0 para Windows x64, de https://developers.google.com/speed/webp/download.
# Google firma cada archivo con PGP («WebP release signing key», 6B0E 6B70 976D E303 EDF2
# F601 F9C3 D6BD B823 2B5D); la suma de abajo es la del archivo con firma verificada.
# Para cambiar de versión: bajar el .zip y su .asc, verificar con gpg y actualizar ambos valores.
$libwebp = 'libwebp-1.6.0-windows-x64'
$libwebpSha = '48886f506b21f62e4661f0f4cbfca19800897c385128e8902542d29a950c93f1'
$libwebpUrl = "https://storage.googleapis.com/downloads.webmproject.org/releases/webp/$libwebp.zip"
$licencia = 'https://raw.githubusercontent.com/webmproject/libwebp/v1.6.0'
$cache = Join-Path $PSScriptRoot 'componentes'

if (-not (Test-Path (Join-Path $cache 'cwebp.exe')) -or -not (Test-Path (Join-Path $cache 'LICENSE-libwebp.txt'))) {
    Write-Host 'Descargando el motor WebP...'
    $tmp = Join-Path ([IO.Path]::GetTempPath()) "pngwebp-comp-$([guid]::NewGuid())"
    New-Item -ItemType Directory $tmp | Out-Null
    try {
        $zip = Join-Path $tmp "$libwebp.zip"
        Invoke-WebRequest -UseBasicParsing $libwebpUrl -OutFile $zip
        if ((Get-FileHash -Algorithm SHA256 $zip).Hash.ToLower() -ne $libwebpSha) { throw "La suma SHA-256 de $libwebp.zip no coincide" }
        Expand-Archive $zip $tmp
        if (Test-Path $cache) { Remove-Item -Recurse -Force $cache }
        New-Item -ItemType Directory $cache | Out-Null
        Copy-Item (Join-Path $tmp "$libwebp\bin\cwebp.exe") $cache
        # Licencia BSD de libwebp y su concesión de patentes, que deben acompañar al binario.
        $texto = (Invoke-WebRequest -UseBasicParsing "$licencia/COPYING").Content + "`n`n" + (Invoke-WebRequest -UseBasicParsing "$licencia/PATENTS").Content
        Set-Content -Path (Join-Path $cache 'LICENSE-libwebp.txt') -Value $texto -Encoding utf8
    } finally {
        Remove-Item -Recurse -Force $tmp -ErrorAction SilentlyContinue
    }
}
$verMotor = (& (Join-Path $cache 'cwebp.exe') -version | Select-Object -First 1).Trim()
Write-Host "Motor WebP $verMotor"

# ---------- Ejecutable ----------
# Icono, manifiesto y datos de versión dentro del .exe
& $winres simply --arch amd64 --out rsrc --manifest gui `
    --icon build\app.ico `
    --product-name PNGWebp --file-description 'PNGWebp - convierte imágenes a WebP' `
    --product-version $version --file-version $version `
    --original-filename PNGWebp.exe --copyright PNGWebp
if ($LASTEXITCODE) { throw 'go-winres falló' }

New-Item -ItemType Directory -Force dist | Out-Null
& $goExe build -trimpath -ldflags '-H windowsgui -s -w' -o dist\PNGWebp.exe .
if ($LASTEXITCODE) { throw 'go build falló' }

# La app busca el motor en la carpeta "componentes" junto a su .exe.
$destino = Join-Path $raiz 'dist\componentes'
if (Test-Path $destino) { Remove-Item -Recurse -Force $destino }
New-Item -ItemType Directory $destino | Out-Null
Copy-Item (Join-Path $cache '*') $destino

# ---------- Instalador ----------
& $makensis /V2 "/DVERSION=$version" build\instalador.nsi
if ($LASTEXITCODE) { throw 'makensis falló' }

Get-ChildItem dist -File | Format-Table Name, @{n = 'MB'; e = { [math]::Round($_.Length / 1MB, 2) } }
