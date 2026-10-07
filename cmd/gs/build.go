package main

import (
	"archive/tar"
	"archive/zip"
	"bufio"
	"bytes"
	"compress/gzip"
	"debug/elf"
	"debug/macho"
	"debug/pe"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"

	"gambiarrascript/ast"
	"gambiarrascript/compiler"
	"gambiarrascript/lexer"
	"gambiarrascript/object"
	"gambiarrascript/parser"
)

// gs build: copia um binario gs e anexa a fonte no final (formato em
// buildMagic). Sem --alvo usa o proprio executavel. Com --alvo de outra
// plataforma baixa o gs da release da MESMA versao (Versao) pra aquele
// os/arch, confere com o checksums.txt e guarda em cache; --gs-base aponta
// um binario do alvo na mao (offline, build de dev, plataforma sem release).

// baseReleases e de onde vem os binarios publicados; var pros testes, e o
// env GS_RELEASES_URL troca (espelho).
var baseReleases = "https://github.com/erikomis/gambiarrascript/releases/download"

// dirCacheUsuario e var pros testes nao sujarem o cache de verdade.
var dirCacheUsuario = os.UserCacheDir

// alvosRelease sao as plataformas que o scripts/release publica.
var alvosRelease = []string{"darwin/amd64", "darwin/arm64", "linux/amd64", "linux/arm64", "windows/amd64"}

// versao de release: 1.2.3 ou 1.2.3-rc1. "dev", "0.4.0+sujo" etc nao.
var reVersaoRelease = regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.]+)?$`)

const limiteRelease = 200 << 20

type opcoesBuild struct {
	arquivo, saida, alvo, gsBase string
}

const usoBuild = "uso: gs build <arquivo.gs> [-o saida] [--alvo os/arch] [--gs-base gs-do-alvo]"

func parseArgsBuild(args []string) (opcoesBuild, error) {
	var o opcoesBuild
	for i := 0; i < len(args); i++ {
		a := args[i]
		valor := func(flag string) (string, error) {
			if v, ok := strings.CutPrefix(a, flag+"="); ok {
				return v, nil
			}
			if i+1 >= len(args) {
				return "", fmt.Errorf("%s precisa de um valor", flag)
			}
			i++
			return args[i], nil
		}
		var err error
		switch {
		case a == "-o":
			o.saida, err = valor("-o")
		case a == "--alvo" || strings.HasPrefix(a, "--alvo="):
			o.alvo, err = valor("--alvo")
		case a == "--gs-base" || strings.HasPrefix(a, "--gs-base="):
			o.gsBase, err = valor("--gs-base")
		case strings.HasPrefix(a, "-"):
			err = fmt.Errorf("flag desconhecida: %s", a)
		case o.arquivo == "":
			o.arquivo = a
		default:
			err = fmt.Errorf("argumento sobrando: %s", a)
		}
		if err != nil {
			return o, err
		}
	}
	if o.arquivo == "" {
		return o, errors.New("faltou o arquivo .gs")
	}
	return o, nil
}

func cmdBuild(args []string) {
	o, err := parseArgsBuild(args)
	if err != nil {
		fmt.Println(err.Error())
		fmt.Println(usoBuild)
		os.Exit(1)
	}
	if _, err := construir(o, os.Stdout); err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
	}
}

// construir gera o binario e devolve o caminho da saida.
func construir(o opcoesBuild, w io.Writer) (string, error) {
	goos, goarch := runtime.GOOS, runtime.GOARCH
	if o.alvo != "" {
		partes := strings.Split(o.alvo, "/")
		if len(partes) != 2 || partes[0] == "" || partes[1] == "" {
			return "", fmt.Errorf("--alvo e os/arch, tipo linux/amd64 (tem: %s)", strings.Join(alvosRelease, ", "))
		}
		goos, goarch = partes[0], partes[1]
	}
	nativo := goos == runtime.GOOS && goarch == runtime.GOARCH
	if o.gsBase == "" && !nativo && !contem(alvosRelease, goos+"/"+goarch) {
		return "", fmt.Errorf("alvo %s/%s nao tem gs publicado (tem: %s); passa um com --gs-base",
			goos, goarch, strings.Join(alvosRelease, ", "))
	}

	fonte, err := os.ReadFile(o.arquivo)
	if err != nil {
		return "", fmt.Errorf("nao consegui abrir %q: %v", o.arquivo, err)
	}
	// valida antes de embedar — binario com script quebrado e vacilo
	p := parser.New(lexer.New(string(fonte)))
	prog := p.ParseProgram()
	if errs := p.Errors(); len(errs) != 0 {
		return "", fmt.Errorf("teu script tem perrengue de parse, arruma antes de buildar:\n  - %s", strings.Join(errs, "\n  - "))
	}
	payload, magic, nModulos := montaPayload(prog, o.arquivo, fonte, w)

	var base []byte
	switch {
	case o.gsBase != "":
		if base, err = os.ReadFile(o.gsBase); err != nil {
			return "", fmt.Errorf("nao consegui ler o --gs-base: %v", err)
		}
	case nativo:
		eu, err := os.Executable()
		if err != nil {
			return "", fmt.Errorf("nao achei o proprio gs: %v", err)
		}
		if base, err = os.ReadFile(eu); err != nil {
			return "", fmt.Errorf("nao consegui ler o proprio gs: %v", err)
		}
	default:
		if base, err = obtemBaseRelease(Versao, goos, goarch, w); err != nil {
			return "", err
		}
	}
	if err := confereExecutavel(base, goos, goarch); err != nil {
		return "", err
	}

	saida := o.saida
	if saida == "" {
		saida = strings.TrimSuffix(filepath.Base(o.arquivo), ".gs")
		if goos == "windows" {
			saida += ".exe"
		}
	}

	out := make([]byte, 0, len(base)+len(payload)+16)
	out = append(out, base...)
	out = append(out, payload...)
	var lenBuf [8]byte
	binary.LittleEndian.PutUint64(lenBuf[:], uint64(len(payload)))
	out = append(out, lenBuf[:]...)
	out = append(out, []byte(magic)...)
	if err := os.WriteFile(saida, out, 0755); err != nil {
		return "", fmt.Errorf("nao consegui escrever a saida: %v", err)
	}

	// macOS (Apple Silicon) mata binario com assinatura invalida; re-assina
	// ad-hoc. Sem codesign no PATH (ou buildando de outro sistema), so avisa.
	if goos == "darwin" {
		if runtime.GOOS != "darwin" {
			fmt.Fprintln(w, "aviso: binario de macOS gerado fora do macOS; se ele for morto, roda la: codesign -s - "+saida)
		} else if err := exec.Command("codesign", "--force", "-s", "-", saida).Run(); err != nil {
			fmt.Fprintln(w, "aviso: nao consegui re-assinar (codesign): "+err.Error())
			fmt.Fprintln(w, "       se o binario for morto pelo macOS, roda: codesign -s - "+saida)
		}
	}
	extra := ""
	if nModulos > 0 {
		extra = fmt.Sprintf(", %d modulo(s) embutido(s)", nModulos)
	}
	fmt.Fprintf(w, "  %s  (binario standalone %s/%s, %.1f MB%s)\n", saida, goos, goarch, float64(len(out))/1024/1024, extra)
	return saida, nil
}

// montaPayload decide o que vai no fim do binario. Script sem importa vai
// cru (GSEMBED1, o formato de sempre). Com importa, vai o pacote JSON com as
// fontes dos modulos (GSEMBED2) — descobertos compilando o programa, que
// resolve os importa (inclusive gs_modulos/) igual na hora de rodar. Vale
// igual pro build nativo e pro --alvo: o payload nao depende da plataforma.
func montaPayload(prog *ast.Program, arquivo string, fonte []byte, w io.Writer) ([]byte, string, int) {
	abs, err := filepath.Abs(arquivo)
	if err != nil {
		abs = arquivo
	}
	comp := compiler.New()
	comp.Arquivo = abs
	if err := comp.Compile(prog); err != nil {
		// a VM nao compilou (o binario cai pro tree-walker): sem a lista de
		// modulos, o importa resolve no disco de quem rodar
		fmt.Fprintln(w, "aviso: nao compilou pra VM, modulos nao foram embutidos: "+err.Error())
		return fonte, buildMagic, 0
	}
	caminhos := comp.Bytecode().Modulos
	if len(caminhos) == 0 {
		return fonte, buildMagic, 0
	}
	pacote := pacoteBuild{Principal: abs, Fonte: string(fonte), Modulos: map[string]string{}}
	for _, c := range caminhos {
		b, err := object.LeModulo(c)
		if err != nil {
			fmt.Fprintln(w, "aviso: nao consegui ler o modulo "+c+", ficou de fora: "+err.Error())
			continue
		}
		pacote.Modulos[c] = string(b)
	}
	payload, err := json.Marshal(pacote)
	if err != nil {
		return fonte, buildMagic, 0
	}
	return payload, buildMagicPacote, len(pacote.Modulos)
}

func contem(lista []string, s string) bool {
	for _, x := range lista {
		if x == s {
			return true
		}
	}
	return false
}

// erroSemRelease explica o que fazer quando nao da pra baixar o gs do alvo.
func erroSemRelease(versao, goos, goarch, motivo string) error {
	return fmt.Errorf("nao tem gs %q publicado pra baixar o binario de %s/%s (%s).\n"+
		"  build de desenvolvimento? usa um gs de release (https://github.com/erikomis/gambiarrascript/releases)\n"+
		"  ou passa o gs do alvo na mao: gs build app.gs --alvo %s/%s --gs-base caminho/do/gs-%s-%s",
		versao, goos, goarch, motivo, goos, goarch, goos, goarch)
}

// obtemBaseRelease devolve o executavel gs <versao> de goos/goarch: do cache
// (<UserCacheDir>/gambiarrascript/<versao>/) ou da release, sempre conferindo
// o pacote com o checksums.txt.
func obtemBaseRelease(versao, goos, goarch string, w io.Writer) ([]byte, error) {
	if !reVersaoRelease.MatchString(versao) {
		return nil, erroSemRelease(versao, goos, goarch, "versao nao e de release")
	}
	ext, bin := ".tar.gz", "gs"
	if goos == "windows" {
		ext, bin = ".zip", "gs.exe"
	}
	pacote := fmt.Sprintf("gs_%s_%s_%s%s", versao, goos, goarch, ext)
	base := baseReleases
	if env := os.Getenv("GS_RELEASES_URL"); env != "" {
		base = env
	}
	base = strings.TrimRight(base, "/") + "/v" + versao

	dirV := ""
	if d, err := dirCacheUsuario(); err == nil {
		dirV = filepath.Join(d, "gambiarrascript", versao)
	} else {
		fmt.Fprintln(w, "aviso: sem diretorio de cache, vou baixar sem guardar")
	}
	doCache := func(nome string) []byte {
		if dirV == "" {
			return nil
		}
		b, err := os.ReadFile(filepath.Join(dirV, nome))
		if err != nil {
			return nil
		}
		return b
	}
	baixaRelease := func(nome string) ([]byte, error) {
		b, err := baixa(base+"/"+nome, false, limiteRelease)
		var es erroStatus
		if errors.As(err, &es) && es.Codigo == 404 {
			return nil, erroSemRelease(versao, goos, goarch, nome+" nao existe na release v"+versao)
		}
		return b, err
	}

	somas := doCache("checksums.txt")
	somasDoCache := somas != nil
	if somas == nil {
		var err error
		if somas, err = baixaRelease("checksums.txt"); err != nil {
			return nil, err
		}
	}
	esperado, ok := procuraChecksum(somas, pacote)
	if !ok {
		return nil, fmt.Errorf("o checksums.txt da v%s nao lista %s — recusado", versao, pacote)
	}

	conteudo := doCache(pacote)
	if conteudo != nil && sha256Hex(conteudo) == esperado {
		fmt.Fprintf(w, "  usando %s do cache (%s)\n", pacote, dirV)
	} else {
		fmt.Fprintf(w, "  baixando %s/%s\n", base, pacote)
		var err error
		if conteudo, err = baixaRelease(pacote); err != nil {
			return nil, err
		}
		if h := sha256Hex(conteudo); h != esperado {
			return nil, fmt.Errorf("RECUSADO: %s nao bate com o checksums.txt (esperado %s, veio %s) — download corrompido ou adulterado", pacote, esperado, h)
		}
		if dirV != "" {
			if err := os.MkdirAll(dirV, 0755); err == nil {
				if !somasDoCache {
					os.WriteFile(filepath.Join(dirV, "checksums.txt"), somas, 0644)
				}
				os.WriteFile(filepath.Join(dirV, pacote), conteudo, 0644)
			}
		}
	}
	return extraiBinario(conteudo, ext, bin)
}

// procuraChecksum acha o sha256 de `arquivo` num checksums.txt (formato do
// sha256sum: "<hex>  <nome>").
func procuraChecksum(somas []byte, arquivo string) (string, bool) {
	sc := bufio.NewScanner(bytes.NewReader(somas))
	for sc.Scan() {
		campos := strings.Fields(sc.Text())
		if len(campos) == 2 && strings.TrimPrefix(campos[1], "*") == arquivo && len(campos[0]) == 64 {
			return strings.ToLower(campos[0]), true
		}
	}
	return "", false
}

// extraiBinario tira o gs (ou gs.exe) de dentro do .tar.gz/.zip da release.
func extraiBinario(pacote []byte, ext, bin string) ([]byte, error) {
	if ext == ".zip" {
		zr, err := zip.NewReader(bytes.NewReader(pacote), int64(len(pacote)))
		if err != nil {
			return nil, fmt.Errorf("zip da release invalido: %v", err)
		}
		for _, f := range zr.File {
			if path.Base(f.Name) != bin || f.FileInfo().IsDir() {
				continue
			}
			rc, err := f.Open()
			if err != nil {
				return nil, err
			}
			defer rc.Close()
			return lerLimitado(rc)
		}
		return nil, fmt.Errorf("o zip da release nao tem %s", bin)
	}
	gz, err := gzip.NewReader(bytes.NewReader(pacote))
	if err != nil {
		return nil, fmt.Errorf("tar.gz da release invalido: %v", err)
	}
	tr := tar.NewReader(gz)
	for {
		h, err := tr.Next()
		if err == io.EOF {
			return nil, fmt.Errorf("o tar.gz da release nao tem %s", bin)
		}
		if err != nil {
			return nil, fmt.Errorf("tar.gz da release invalido: %v", err)
		}
		if h.Typeflag == tar.TypeReg && path.Base(h.Name) == bin {
			return lerLimitado(tr)
		}
	}
}

func lerLimitado(r io.Reader) ([]byte, error) {
	b, err := io.ReadAll(io.LimitReader(r, limiteRelease+1))
	if err != nil {
		return nil, err
	}
	if len(b) > limiteRelease {
		return nil, errors.New("binario da release grande demais")
	}
	return b, nil
}

// confereExecutavel checa que a base e mesmo um executavel do os/arch pedido
// (pega --gs-base trocado antes de gerar um binario que nao roda).
func confereExecutavel(b []byte, goos, goarch string) error {
	r := bytes.NewReader(b)
	errFormato := fmt.Errorf("o gs base nao e um executavel de %s/%s", goos, goarch)
	arch := ""
	switch goos {
	case "linux":
		f, err := elf.NewFile(r)
		if err != nil {
			return errFormato
		}
		arch = map[elf.Machine]string{elf.EM_X86_64: "amd64", elf.EM_AARCH64: "arm64", elf.EM_386: "386", elf.EM_ARM: "arm"}[f.Machine]
	case "windows":
		f, err := pe.NewFile(r)
		if err != nil {
			return errFormato
		}
		arch = map[uint16]string{pe.IMAGE_FILE_MACHINE_AMD64: "amd64", pe.IMAGE_FILE_MACHINE_ARM64: "arm64", pe.IMAGE_FILE_MACHINE_I386: "386"}[f.Machine]
	case "darwin":
		f, err := macho.NewFile(r)
		if err != nil {
			return errFormato
		}
		arch = map[macho.Cpu]string{macho.CpuAmd64: "amd64", macho.CpuArm64: "arm64"}[f.Cpu]
	default:
		return nil // sistema que nao sei conferir: confia no --gs-base
	}
	if arch != goarch {
		return fmt.Errorf("%s (a arquitetura do gs base e %q)", errFormato, arch)
	}
	return nil
}
