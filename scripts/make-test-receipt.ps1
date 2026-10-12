# Merender gambar struk dummy untuk pengujian OCR (tanpa dependensi).
Add-Type -AssemblyName System.Drawing

$width = 420
$lineHeight = 34
$lines = @(
    "WARUNG MAKAN BAHAGIA",
    "Jl. Merdeka No. 12, Bandung",
    "--------------------------------",
    "12/10/2026 14:32   No: 0081",
    "--------------------------------",
    "Nasi Goreng      x1   25.000",
    "Es Teh Manis     x2   10.000",
    "Kerupuk          x1   12.500",
    "--------------------------------",
    "SUBTOTAL              47.500",
    "PB1 11%                5.225",
    "TOTAL                 52.725",
    "TUNAI                100.000",
    "KEMBALI               47.275",
    "--------------------------------",
    "Terima kasih!"
)

$height = ($lines.Count + 2) * $lineHeight + 40
$bmp = New-Object System.Drawing.Bitmap($width, $height)
$g = [System.Drawing.Graphics]::FromImage($bmp)
$g.Clear([System.Drawing.Color]::White)
$g.TextRenderingHint = [System.Drawing.Text.TextRenderingHint]::ClearTypeGridFit
$font = New-Object System.Drawing.Font("Consolas", 15)
$brush = [System.Drawing.Brushes]::Black

$y = 20
foreach ($line in $lines) {
    $g.DrawString($line, $font, $brush, 20.0, [float]$y)
    $y += $lineHeight
}

$g.Dispose()
$out = Join-Path $env:TEMP "caupocket-receipt-test.png"
$bmp.Save($out, [System.Drawing.Imaging.ImageFormat]::Png)
$bmp.Dispose()
Write-Output $out
