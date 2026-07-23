$base = 'http://localhost:8080/api/v1'
$login = Invoke-RestMethod -Uri "$base/auth/login" -Method Post -ContentType 'application/json' -Body (@{ correo = 'admin@syncbox.co'; password = 'Cambiar.123!' } | ConvertTo-Json)
$H = @{ Authorization = "Bearer $($login.access_token)" }
$cats = Invoke-RestMethod -Uri "$base/categorias" -Headers $H
$estados = Invoke-RestMethod -Uri "$base/estados" -Headers $H
$body = @{
    nombre       = 'Verifica Activo'
    categoria_id = $cats[0].id
    estado_id    = ($estados | Where-Object { $_.nombre -eq 'Disponible' }).id
    serial       = "SN-$(Get-Random)"
} | ConvertTo-Json
$eq = Invoke-RestMethod -Uri "$base/equipos" -Method Post -Headers $H -ContentType 'application/json' -Body $body
$eq | ConvertTo-Json -Depth 4
