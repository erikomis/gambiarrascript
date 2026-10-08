# Instalador do GambiarraScript (gs) pra Windows (PowerShell 5.1+ ou pwsh).
#
#   irm https://raw.githubusercontent.com/erikomis/gambiarrascript/main/install.ps1 | iex
#
# Variaveis opcionais (defina antes de rodar):
#   $env:GS_VERSAO = "0.5.0"      instala essa versao em vez da ultima
#   $env:GS_DIR = "C:\ferramentas" instala nesse diretorio (padrao:
#                                 %LOCALAPPDATA%\Programs\gambiarrascript)
#   $env:GS_URL_BASE = "..."      baixa os arquivos dessa URL base em vez do
#                                 GitHub (pra testes/espelhos; exige GS_VERSAO)
#   $env:GS_SEM_PATH = "1"        nao mexe no PATH do usuario
#   $env:GS_SEM_API = "1"         nao usa a API do GitHub pra achar a ultima
#                                 versao (vai direto pelo redirect; pra teste)
#
# Nunca pede admin: instala na pasta do usuario e poe no PATH do USUARIO.
# Tudo dentro de uma funcao pra um download cortado no meio nao rodar pela metade.

function Instala-Gs {
    $ErrorActionPreference = 'Stop'
    $ProgressPreference = 'SilentlyContinue' # a barra de progresso deixa o download 10x mais lento
    $repo = 'erikomis/gambiarrascript'

    function Diz($msg) { Write-Host "gs: $msg" }
    function Morre($msg) { throw "gs: deu ruim: $msg" }

    # TLS 1.2 no PowerShell 5.1 (o padrao dele ainda e TLS 1.0)
    try { [Net.ServicePointManager]::SecurityProtocol = [Net.ServicePointManager]::SecurityProtocol -bor [Net.SecurityProtocolType]::Tls12 } catch {}

    # so tem binario windows/amd64 nas releases; arm64 roda ele emulado
    $arq = 'amd64'
    if ($env:PROCESSOR_ARCHITECTURE -eq 'x86') { Morre 'Windows 32 bits nao tem binario - so 64 bits' }

    $versao = $env:GS_VERSAO
    if ($env:GS_URL_BASE) {
        if (-not $versao) { Morre 'GS_URL_BASE precisa de GS_VERSAO junto' }
        $base = $env:GS_URL_BASE.TrimEnd('/')
    } else {
        if (-not $versao) {
            if (-not $env:GS_SEM_API) {
                try {
                    $versao = (Invoke-RestMethod -UseBasicParsing "https://api.github.com/repos/$repo/releases/latest").tag_name
                } catch {}
            }
            if (-not $versao) {
                # API sem login tem limite por IP (rede de empresa, CI...): o
                # redirect de /releases/latest diz a tag sem passar pela API.
                # PowerShell 5.1 e 7 expoem o cabecalho de jeitos diferentes.
                $local = $null
                $resp = $null
                try {
                    $resp = Invoke-WebRequest -UseBasicParsing -Method Head -MaximumRedirection 0 -ErrorAction Stop "https://github.com/$repo/releases/latest"
                } catch {
                    $resp = $_.Exception.Response
                }
                if ($resp) {
                    try { $local = $resp.Headers.Location } catch {}
                    if (-not $local) { try { $local = $resp.Headers['Location'] } catch {} }
                }
                if ($local -and ([string]$local) -match '/releases/tag/([^/]+)$') { $versao = $Matches[1] }
            }
            if (-not $versao) {
                Morre 'nao consegui descobrir a ultima versao no GitHub (nenhuma release publicada? sem internet?). Tenta $env:GS_VERSAO = "x.y.z"'
            }
        }
        $versao = $versao.TrimStart('v')
        $base = "https://github.com/$repo/releases/download/v$versao"
    }
    $versao = $versao.TrimStart('v')
    $nome = "gs_${versao}_windows_$arq.zip"

    $tmp = Join-Path ([IO.Path]::GetTempPath()) ("gs-instala-" + [Guid]::NewGuid().ToString('N'))
    New-Item -ItemType Directory -Path $tmp | Out-Null
    try {
        Diz "baixando $nome (v$versao)..."
        $zip = Join-Path $tmp $nome
        $sums = Join-Path $tmp 'checksums.txt'
        try {
            Invoke-WebRequest -UseBasicParsing -Uri "$base/$nome" -OutFile $zip
            Invoke-WebRequest -UseBasicParsing -Uri "$base/checksums.txt" -OutFile $sums
        } catch {
            Morre "download falhou ($base/$nome): $($_.Exception.Message)"
        }

        # confere o sha256 contra o checksums.txt da release
        # laco simples em vez de Where-Object | Select-Object -First: o -First
        # interrompe o pipeline e, com ErrorActionPreference=Stop, estoura
        # "Object reference not set" no pwsh
        $linha = $null
        foreach ($l in [IO.File]::ReadAllLines($sums)) {
            $partes = $l.Trim() -split '\s+'
            if ($partes.Count -ge 2 -and $partes[-1].TrimStart('*') -eq $nome) { $linha = $l; break }
        }
        if (-not $linha) { Morre "o checksums.txt nao tem $nome" }
        $esperado = ($linha -split '\s+')[0].ToLower()
        $veio = (Get-FileHash -Algorithm SHA256 $zip).Hash.ToLower()
        if ($esperado -ne $veio) {
            Morre "sha256 nao confere - download corrompido ou adulterado. Nada foi instalado.`n  esperado: $esperado`n  veio:     $veio"
        }
        Diz 'sha256 conferido'

        $dir = $env:GS_DIR
        if (-not $dir) { $dir = Join-Path $env:LOCALAPPDATA 'Programs\gambiarrascript' }
        New-Item -ItemType Directory -Force -Path $dir | Out-Null

        $extraido = Join-Path $tmp 'x'
        Expand-Archive -Path $zip -DestinationPath $extraido -Force
        $exe = @(Get-ChildItem -Path $extraido -Recurse -Filter 'gs.exe')
        if ($exe.Count -eq 0) { Morre "o $nome nao tem gs.exe dentro" }
        $exe = $exe[0]
        # copia pra um nome temporario e troca: um gs.exe aberto nao fica pela metade
        $destino = Join-Path $dir 'gs.exe'
        $novo = Join-Path $dir 'gs.exe.novo'
        Copy-Item $exe.FullName $novo -Force
        Move-Item $novo $destino -Force
        Diz "instalado em $destino"

        if (-not $env:GS_SEM_PATH) {
            $pathUsuario = [Environment]::GetEnvironmentVariable('Path', 'User')
            $partes = @()
            if ($pathUsuario) { $partes = $pathUsuario.Split(';') | Where-Object { $_ } }
            if ($partes -notcontains $dir) {
                [Environment]::SetEnvironmentVariable('Path', (($partes + $dir) -join ';'), 'User')
                Diz "botei $dir no PATH do teu usuario - abre um terminal novo pra valer"
            }
            if (($env:Path.Split(';')) -notcontains $dir) { $env:Path = "$env:Path;$dir" }
        }

        # $IsWindows nao existe no PowerShell 5.1 (que so roda no Windows)
        if ($null -eq $IsWindows -or $IsWindows) { & $destino --version }
        Diz "pronto! roda 'gs repl' e bora gambiarrar"
    } finally {
        Remove-Item -Recurse -Force $tmp -ErrorAction SilentlyContinue
    }
}

Instala-Gs
