# Load environment variables from .env.production and build Docker image

$envFile = Join-Path $PSScriptRoot ".env.production"

if (Test-Path $envFile) {
    Write-Host "Loading environment variables from .env.production..." -ForegroundColor Green
    Get-Content $envFile | ForEach-Object {
        if ($_ -match '^([^=]+)=(.*)$') {
            $key = $matches[1]
            $value = $matches[2]
            [Environment]::SetEnvironmentVariable($key, $value, 'Process')
            Write-Host "  $key = $value" -ForegroundColor Gray
        }
    }
    Write-Host ""
} else {
    Write-Host "Warning: .env.production not found!" -ForegroundColor Yellow
}

Write-Host "Building Docker image..." -ForegroundColor Cyan
docker compose build $args

