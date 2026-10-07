$ErrorActionPreference = "Stop"

# Render every maintained PlantUML source in both repository formats.
$projectRoot = (Resolve-Path (Join-Path $PSScriptRoot "..")).Path
$mount = "${projectRoot}:/workspace"
$image = "plantuml/plantuml:1.2026.8"
$sources = @("docs/architecture", "docs/cjm")

foreach ($format in @("svg", "png")) {
    & docker run --rm -v $mount -w /workspace $image `
        -charset UTF-8 "-t$format" -o ../rendered @sources
    if ($LASTEXITCODE -ne 0) {
        throw "PlantUML failed while rendering $format"
    }
}

Write-Host "Diagrams rendered to docs/rendered"
