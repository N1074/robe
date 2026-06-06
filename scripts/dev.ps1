param(
    [Parameter(Mandatory = $true)]
    [ValidateSet("run", "google-auth", "build", "check", "fmt", "test", "vet", "health")]
    [string]$Task
)

$ErrorActionPreference = "Stop"
$Root = Split-Path -Parent $PSScriptRoot
Set-Location $Root

switch ($Task) {
    "run" {
        go run ./cmd/robe-server
    }
    "google-auth" {
        go run ./cmd/robe-google-auth
    }
    "build" {
        New-Item -ItemType Directory -Force bin | Out-Null
        go build -o bin/robe-server.exe ./cmd/robe-server
        go build -o bin/robe-google-auth.exe ./cmd/robe-google-auth
    }
    "fmt" {
        gofmt -w ./cmd ./internal
    }
    "test" {
        go test ./...
    }
    "vet" {
        go vet ./...
    }
    "check" {
        gofmt -w ./cmd ./internal
        go test ./...
        go vet ./...
    }
    "health" {
        Invoke-RestMethod http://localhost:8080/health
    }
}
