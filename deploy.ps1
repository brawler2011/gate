# Deploy frontend Docker image to remote server

param(
    [string]$Server = "217.12.38.213",
    [string]$User = "root",
    [string]$ImageName = "gate149-frontend",
    [string]$ArchiveName = "gate149-frontend.tar.gz",
    [switch]$AutoDeploy = $false,
    [switch]$KeepArchive = $false,
    [ValidateSet("build", "save", "compress", "upload", "deploy")]
    [string]$StartFrom = "build"
)

$ErrorActionPreference = "Stop"
$currentStep = ""
$scriptSuccess = $false

# Temporary files to clean up
$tarPath = "$PSScriptRoot\gate149-frontend.tar"
$gzPath = "$PSScriptRoot\$ArchiveName"

# Cleanup function
function Cleanup-TempFiles {
    param([bool]$Force = $false)
    
    if ($KeepArchive -and -not $Force) {
        Write-Host "Keeping archive files (-KeepArchive flag is set)" -ForegroundColor Yellow
        return
    }
    
    $cleaned = $false
    if (Test-Path $tarPath) {
        Remove-Item $tarPath -Force -ErrorAction SilentlyContinue
        Write-Host "  Removed: $tarPath" -ForegroundColor Gray
        $cleaned = $true
    }
    if (Test-Path $gzPath) {
        Remove-Item $gzPath -Force -ErrorAction SilentlyContinue
        Write-Host "  Removed: $gzPath" -ForegroundColor Gray
        $cleaned = $true
    }
    
    if ($cleaned) {
        Write-Host "Cleanup complete." -ForegroundColor Yellow
    }
}

# Error handler with helpful messages
function Show-ErrorHelp {
    param([string]$Step, [string]$ErrorMessage)
    
    Write-Host ""
    Write-Host "========================================" -ForegroundColor Red
    Write-Host "ERROR: Deployment failed!" -ForegroundColor Red
    Write-Host "========================================" -ForegroundColor Red
    Write-Host ""
    Write-Host "Failed at: $Step" -ForegroundColor Red
    Write-Host "Error: $ErrorMessage" -ForegroundColor Red
    Write-Host ""
    
    switch ($Step) {
        "build" {
            Write-Host "What happened:" -ForegroundColor Yellow
            Write-Host "  Docker image build failed"
            Write-Host ""
            Write-Host "Possible causes:" -ForegroundColor Yellow
            Write-Host "  - Dockerfile syntax error"
            Write-Host "  - Missing dependencies"
            Write-Host "  - .env.production not found or invalid"
            Write-Host "  - Docker daemon not running"
            Write-Host ""
            Write-Host "How to fix:" -ForegroundColor Cyan
            Write-Host "  1. Check Docker is running: docker ps"
            Write-Host "  2. Check .env.production exists and has correct values"
            Write-Host "  3. Try building manually: docker compose build"
            Write-Host "  4. Fix errors and run: .\deploy.ps1"
        }
        "save" {
            Write-Host "What happened:" -ForegroundColor Yellow
            Write-Host "  Could not save Docker image to tar file"
            Write-Host ""
            Write-Host "Possible causes:" -ForegroundColor Yellow
            Write-Host "  - Image does not exist (build might have failed silently)"
            Write-Host "  - Not enough disk space"
            Write-Host "  - Permission denied"
            Write-Host ""
            Write-Host "How to fix:" -ForegroundColor Cyan
            Write-Host "  1. Check image exists: docker images | findstr frontend"
            Write-Host "  2. Check disk space"
            Write-Host "  3. Rebuild and try again: .\deploy.ps1"
        }
        "compress" {
            Write-Host "What happened:" -ForegroundColor Yellow
            Write-Host "  Could not compress tar file to gzip"
            Write-Host ""
            Write-Host "Possible causes:" -ForegroundColor Yellow
            Write-Host "  - Tar file not found (previous step failed)"
            Write-Host "  - Not enough disk space"
            Write-Host "  - 7zip not installed and .NET compression failed"
            Write-Host ""
            Write-Host "How to fix:" -ForegroundColor Cyan
            Write-Host "  1. Install 7zip: https://www.7-zip.org/"
            Write-Host "  2. Or check disk space"
            Write-Host "  3. Run from save step: .\deploy.ps1 -StartFrom save"
        }
        "upload" {
            Write-Host "What happened:" -ForegroundColor Yellow
            Write-Host "  Could not upload archive to server"
            Write-Host ""
            Write-Host "Possible causes:" -ForegroundColor Yellow
            Write-Host "  - SSH connection failed (wrong credentials or banned IP)"
            Write-Host "  - Network issues"
            Write-Host "  - Server disk full"
            Write-Host "  - Archive file not found"
            Write-Host ""
            Write-Host "How to fix:" -ForegroundColor Cyan
            Write-Host "  1. Test SSH: ssh $User@$Server"
            Write-Host "  2. If banned, wait 15 min or unban from server console"
            Write-Host "  3. Check server disk: ssh $User@$Server 'df -h'"
            Write-Host "  4. Resume upload: .\deploy.ps1 -StartFrom upload"
        }
        "deploy" {
            Write-Host "What happened:" -ForegroundColor Yellow
            Write-Host "  Server deployment script failed"
            Write-Host ""
            Write-Host "Possible causes:" -ForegroundColor Yellow
            Write-Host "  - Docker issues on server"
            Write-Host "  - docker-compose.yaml missing or invalid"
            Write-Host "  - Container failed to start"
            Write-Host ""
            Write-Host "How to fix:" -ForegroundColor Cyan
            Write-Host "  1. SSH to server: ssh $User@$Server"
            Write-Host "  2. Check logs: docker compose logs frontend"
            Write-Host "  3. Try manually: cd /opt/gate/infrastructure && docker compose up -d frontend"
            Write-Host "  4. Or re-run deployment: .\deploy.ps1 -StartFrom deploy -AutoDeploy"
        }
    }
    
    Write-Host ""
    Write-Host "Quick reference - Resume from any step:" -ForegroundColor DarkGray
    Write-Host "  .\deploy.ps1 -StartFrom build" -ForegroundColor DarkGray
    Write-Host "  .\deploy.ps1 -StartFrom save" -ForegroundColor DarkGray
    Write-Host "  .\deploy.ps1 -StartFrom compress" -ForegroundColor DarkGray
    Write-Host "  .\deploy.ps1 -StartFrom upload" -ForegroundColor DarkGray
    Write-Host "  .\deploy.ps1 -StartFrom deploy -AutoDeploy" -ForegroundColor DarkGray
    Write-Host ""
}

# Step mapping
$steps = @{
    "build" = 1
    "save" = 2
    "compress" = 3
    "upload" = 4
    "deploy" = 5
}

$startStep = $steps[$StartFrom]

try {
    # Step 1: Build the Docker image
    if ($startStep -le 1) {
        $currentStep = "build"
        Write-Host "========================================" -ForegroundColor Cyan
        Write-Host "Step 1: Building Docker image..." -ForegroundColor Cyan
        Write-Host "========================================" -ForegroundColor Cyan
        & "$PSScriptRoot\build.ps1"

        if ($LASTEXITCODE -ne 0) {
            throw "Docker build returned exit code $LASTEXITCODE"
        }
        Write-Host ""
    }

    # Step 2: Save Docker image to tar
    if ($startStep -le 2) {
        $currentStep = "save"
        Write-Host "========================================" -ForegroundColor Cyan
        Write-Host "Step 2: Saving Docker image to tar..." -ForegroundColor Cyan
        Write-Host "========================================" -ForegroundColor Cyan

        # Get the actual image name from docker compose
        $composedImageName = "frontend-frontend"

        docker save -o $tarPath $composedImageName

        if ($LASTEXITCODE -ne 0) {
            throw "docker save returned exit code $LASTEXITCODE"
        }
        
        Write-Host "Saved to: $tarPath" -ForegroundColor Green
        Write-Host ""
    }

    # Step 3: Compress with gzip
    if ($startStep -le 3) {
        $currentStep = "compress"
        Write-Host "========================================" -ForegroundColor Cyan
        Write-Host "Step 3: Compressing with gzip..." -ForegroundColor Cyan
        Write-Host "========================================" -ForegroundColor Cyan

        if (-not (Test-Path $tarPath)) {
            throw "Tar file not found: $tarPath"
        }

        # Try using 7zip if available
        $7zipPath = "C:\Program Files\7-Zip\7z.exe"
        if (Test-Path $7zipPath) {
            & $7zipPath a -tgzip $gzPath $tarPath
            if ($LASTEXITCODE -ne 0) {
                throw "7zip returned exit code $LASTEXITCODE"
            }
        } else {
            # Fallback: use .NET compression
            Write-Host "Using .NET compression (7zip not found)..." -ForegroundColor Yellow
            $tarBytes = [System.IO.File]::ReadAllBytes($tarPath)
            $outputStream = [System.IO.File]::Create($gzPath)
            $gzipStream = New-Object System.IO.Compression.GzipStream($outputStream, [System.IO.Compression.CompressionMode]::Compress)
            $gzipStream.Write($tarBytes, 0, $tarBytes.Length)
            $gzipStream.Close()
            $outputStream.Close()
        }

        # Remove uncompressed tar after successful compression
        Remove-Item $tarPath -Force

        $fileSize = (Get-Item $gzPath).Length / 1MB
        Write-Host "Archive created: $ArchiveName ($([math]::Round($fileSize, 2)) MB)" -ForegroundColor Green
        Write-Host ""
    }

    # Step 4: Upload to server
    if ($startStep -le 4) {
        $currentStep = "upload"
        Write-Host "========================================" -ForegroundColor Cyan
        Write-Host "Step 4: Uploading to server..." -ForegroundColor Cyan
        Write-Host "========================================" -ForegroundColor Cyan

        if (-not (Test-Path $gzPath)) {
            throw "Archive not found: $gzPath"
        }

        # Upload image archive via SCP
        $remotePath = "${User}@${Server}:/tmp/$ArchiveName"
        Write-Host "Uploading image to $remotePath" -ForegroundColor Gray

        scp $gzPath $remotePath

        if ($LASTEXITCODE -ne 0) {
            throw "scp returned exit code $LASTEXITCODE"
        }

        # Upload deployment script
        $deployScript = "$PSScriptRoot\server-deploy.sh"
        if (Test-Path $deployScript) {
            Write-Host "Uploading deployment script..." -ForegroundColor Gray
            scp $deployScript "${User}@${Server}:/tmp/server-deploy.sh"

            if ($LASTEXITCODE -eq 0) {
                Write-Host "Deployment script uploaded" -ForegroundColor Green
            }
        }

        Write-Host ""
    }

    # Step 5: Deploy on server
    if ($startStep -le 5) {
        Write-Host "========================================" -ForegroundColor Green
        Write-Host "Upload complete!" -ForegroundColor Green
        Write-Host "========================================" -ForegroundColor Green
        Write-Host ""

        if ($AutoDeploy) {
            $currentStep = "deploy"
            Write-Host "========================================" -ForegroundColor Cyan
            Write-Host "Step 5: Running deployment on server..." -ForegroundColor Cyan
            Write-Host "========================================" -ForegroundColor Cyan
            Write-Host ""

            ssh -o ConnectTimeout=10 "${User}@${Server}" "bash /tmp/server-deploy.sh"

            if ($LASTEXITCODE -ne 0) {
                # Server script already showed its own error, just exit silently
                Write-Host ""
                Write-Host "Server deployment failed. See error above." -ForegroundColor Red
                Write-Host "To retry: .\deploy.ps1 -StartFrom deploy -AutoDeploy" -ForegroundColor Yellow
                exit 1
            }

            Write-Host ""
            Write-Host "========================================" -ForegroundColor Green
            Write-Host "Full deployment complete!" -ForegroundColor Green
            Write-Host "========================================" -ForegroundColor Green
        } else {
            Write-Host "To deploy on the server run:" -ForegroundColor Yellow
            Write-Host ""
            Write-Host "  ssh $User@$Server" -ForegroundColor White
            Write-Host "  bash /tmp/server-deploy.sh" -ForegroundColor White
            Write-Host ""
            Write-Host "Or in one command:" -ForegroundColor Yellow
            $deployCmd = "ssh $User@$Server bash /tmp/server-deploy.sh"
            Write-Host "  $deployCmd" -ForegroundColor Cyan
            Write-Host ""
            Write-Host "Or run with -AutoDeploy:" -ForegroundColor Yellow
            Write-Host "  .\deploy.ps1 -AutoDeploy" -ForegroundColor Cyan
            Write-Host ""
        }
    }

    $scriptSuccess = $true

    Write-Host "Available options:" -ForegroundColor DarkGray
    Write-Host "  .\deploy.ps1 -StartFrom build       # Start from beginning" -ForegroundColor DarkGray
    Write-Host "  .\deploy.ps1 -StartFrom upload      # Skip to upload" -ForegroundColor DarkGray
    Write-Host "  .\deploy.ps1 -KeepArchive           # Keep .tar.gz file after upload" -ForegroundColor DarkGray
    Write-Host "  .\deploy.ps1 -AutoDeploy            # Auto-deploy after upload" -ForegroundColor DarkGray
    Write-Host ""

} catch {
    Show-ErrorHelp -Step $currentStep -ErrorMessage $_.Exception.Message
} finally {
    # This runs even on Ctrl+C (most of the time)
    Write-Host ""
    
    # Check if there are files to clean up
    $hasFiles = (Test-Path $tarPath) -or (Test-Path $gzPath)
    
    if ($scriptSuccess) {
        # Success - clean up archive after upload (unless -KeepArchive)
        if ($hasFiles) {
            if ($KeepArchive) {
                Write-Host "Archive kept at: $gzPath" -ForegroundColor Yellow
            } else {
                Write-Host "Cleaning up..." -ForegroundColor Gray
                Cleanup-TempFiles
            }
        }
    } else {
        # Failed or interrupted - ask user
        if ($hasFiles) {
            Write-Host ""
            Write-Host "Temporary files found:" -ForegroundColor Yellow
            if (Test-Path $tarPath) { Write-Host "  - $tarPath" -ForegroundColor Gray }
            if (Test-Path $gzPath) { Write-Host "  - $gzPath" -ForegroundColor Gray }
            Write-Host ""
            
            $response = Read-Host "Keep these files? (y/N)"
            
            if ($response -eq "y" -or $response -eq "Y") {
                Write-Host "Files preserved." -ForegroundColor Green
                if (Test-Path $gzPath) {
                    Write-Host "Resume with: .\deploy.ps1 -StartFrom upload" -ForegroundColor Cyan
                } elseif (Test-Path $tarPath) {
                    Write-Host "Resume with: .\deploy.ps1 -StartFrom compress" -ForegroundColor Cyan
                }
            } else {
                Write-Host "Cleaning up..." -ForegroundColor Gray
                Cleanup-TempFiles -Force $true
            }
        }
    }
}
