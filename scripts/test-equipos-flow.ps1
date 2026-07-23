$ErrorActionPreference = 'Stop'
$base = 'http://localhost:8080/api/v1'
function Show($t, $o) {
    Write-Host "`n=== $t ===" -ForegroundColor Cyan
    ($o | ConvertTo-Json -Depth 6) | Write-Host
}

# 1) Login
$login = Invoke-RestMethod -Uri "$base/auth/login" -Method Post -ContentType 'application/json' -Body (@{ correo = 'admin@syncbox.co'; password = 'Cambiar.123!' } | ConvertTo-Json)
Show 'LOGIN' $login
$token = $login.access_token
$H = @{ Authorization = "Bearer $token" }

# 2) Categorias y estados
$cats = Invoke-RestMethod -Uri "$base/categorias" -Headers $H
Show 'CATEGORIAS' $cats
$estados = Invoke-RestMethod -Uri "$base/estados" -Headers $H
Show 'ESTADOS' $estados

$catId = $cats[0].id
$estDisp = ($estados | Where-Object { $_.nombre -eq 'Disponible' }).id
$estMant = ($estados | Where-Object { $_.nombre -eq 'Mantenimiento' }).id

# 3) Crear equipo
$body = @{
    nombre       = 'Equipo Prueba API'
    fabricante   = 'ACME'
    modelo       = 'X-1000'
    serial       = "SN-$(Get-Random)"
    categoria_id = $catId
    estado_id    = $estDisp
    ubicacion    = 'Bodega 1'
} | ConvertTo-Json
$eq = Invoke-RestMethod -Uri "$base/equipos" -Method Post -Headers $H -ContentType 'application/json' -Body $body
Show 'CREATE EQUIPO' $eq
$eid = $eq.id

# 4) GET detalle
$det = Invoke-RestMethod -Uri "$base/equipos/$eid" -Headers $H
Show 'GET EQUIPO' $det

# 5) Historial inicial (debe traer 1 entrada de alta)
$hist1 = Invoke-RestMethod -Uri "$base/equipos/$eid/historial" -Headers $H
Show 'HISTORIAL INICIAL' $hist1

# 6) Cambio de estado
$ch = Invoke-RestMethod -Uri "$base/equipos/$eid/estado" -Method Patch -Headers $H -ContentType 'application/json' -Body (@{ estado_id = $estMant; motivo = 'Mantenimiento programado' } | ConvertTo-Json)
Show 'CHANGE STATE' $ch

# 7) Update campos
$upd = Invoke-RestMethod -Uri "$base/equipos/$eid" -Method Patch -Headers $H -ContentType 'application/json' -Body (@{ ubicacion = 'Taller A'; fabricante = 'ACME Industries' } | ConvertTo-Json)
Show 'UPDATE' $upd

# 8) GET ficha (debe ser null o 200 con null)
try {
    $f0 = Invoke-RestMethod -Uri "$base/equipos/$eid/ficha" -Headers $H
    Show 'FICHA INICIAL' $f0
} catch {
    Write-Host "FICHA INICIAL err: $($_.Exception.Message)" -ForegroundColor Yellow
}

# 9) PUT ficha
$ficha = @{ peso = 120.5; potencia = 3.2; dimensiones = '100x60x50 cm'; anio = 2024; observaciones = 'Equipo de prueba'; atributos_extra = @{} } | ConvertTo-Json
$f1 = Invoke-RestMethod -Uri "$base/equipos/$eid/ficha" -Method Put -Headers $H -ContentType 'application/json' -Body $ficha
Show 'UPSERT FICHA' $f1

# 10) Update ficha (cambiar peso y anio) para generar diff
$ficha2 = @{ peso = 130.0; potencia = 3.2; dimensiones = '100x60x50 cm'; anio = 2025; observaciones = 'Equipo de prueba'; atributos_extra = @{} } | ConvertTo-Json
$f2 = Invoke-RestMethod -Uri "$base/equipos/$eid/ficha" -Method Put -Headers $H -ContentType 'application/json' -Body $ficha2
Show 'UPSERT FICHA 2' $f2

# 11) Historial final
$hist2 = Invoke-RestMethod -Uri "$base/equipos/$eid/historial" -Headers $H
Show 'HISTORIAL FINAL' $hist2

# 12) List
$list = Invoke-RestMethod -Uri "$base/equipos?limit=5" -Headers $H
Show 'LIST' $list

Write-Host "`n>>> Equipo creado: $eid" -ForegroundColor Green
