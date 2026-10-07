package main

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"debug/elf"
	"debug/pe"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// elfFalso monta so o cabecalho ELF64 (little endian) da arquitetura pedida:
// suficiente pro debug/elf reconhecer, sem precisar de binario de verdade.
func elfFalso(maquina elf.Machine) []byte {
	var b bytes.Buffer
	b.Write([]byte{0x7f, 'E', 'L', 'F', 2, 1, 1, 0, 0, 0, 0, 0, 0, 0, 0, 0})
	le := binary.LittleEndian
	binary.Write(&b, le, uint16(2)) // ET_EXEC
	binary.Write(&b, le, uint16(maquina))
	binary.Write(&b, le, uint32(1))
	binary.Write(&b, le, uint64(0)) // entry
	binary.Write(&b, le, uint64(0)) // phoff
	binary.Write(&b, le, uint64(0)) // shoff
	binary.Write(&b, le, uint32(0)) // flags
	binary.Write(&b, le, uint16(64))
	binary.Write(&b, le, uint16(56))
	binary.Write(&b, le, uint16(0))
	binary.Write(&b, le, uint16(64))
	binary.Write(&b, le, uint16(0))
	binary.Write(&b, le, uint16(0))
	b.WriteString("corpo-do-gs-falso")
	return b.Bytes()
}

// peFalso monta o minimo de um PE (cabecalho DOS + COFF) pra amd64.
func peFalso() []byte {
	b := make([]byte, 0x40)
	copy(b, "MZ")
	binary.LittleEndian.PutUint32(b[0x3c:], 0x40)
	b = append(b, 'P', 'E', 0, 0)
	coff := make([]byte, 20)
	binary.LittleEndian.PutUint16(coff[0:], pe.IMAGE_FILE_MACHINE_AMD64)
	b = append(b, coff...)
	return append(b, []byte("corpo-do-gs-exe-falso")...)
}

func tarGz(t *testing.T, nome string, conteudo []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for _, a := range []struct {
		n string
		c []byte
	}{{"README.md", []byte("leia")}, {nome, conteudo}, {"LICENSE", []byte("MIT")}} {
		tw.WriteHeader(&tar.Header{Name: a.n, Mode: 0755, Size: int64(len(a.c)), Typeflag: tar.TypeReg})
		tw.Write(a.c)
	}
	tw.Close()
	gz.Close()
	return buf.Bytes()
}

func zipCom(t *testing.T, nome string, conteudo []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	f, _ := zw.Create(nome)
	f.Write(conteudo)
	zw.Close()
	return buf.Bytes()
}

// alvoEstrangeiro escolhe um linux/* diferente da maquina do teste.
func alvoEstrangeiro() (string, elf.Machine) {
	if runtime.GOOS == "linux" && runtime.GOARCH == "amd64" {
		return "linux/arm64", elf.EM_AARCH64
	}
	return "linux/amd64", elf.EM_X86_64
}

// releaseFalsa sobe um servidor de release v<versao> com os pacotes dados e
// o checksums.txt deles, aponta o gs pra ele e isola o cache num TempDir.
func releaseFalsa(t *testing.T, versao string, pacotes map[string][]byte) (*servidorModulos, string) {
	t.Helper()
	s := novoServidorModulos(t)
	var somas strings.Builder
	for nome, c := range pacotes {
		s.poe("/v"+versao+"/"+nome, string(c))
		fmt.Fprintf(&somas, "%s  %s\n", sha256Hex(c), nome)
	}
	s.poe("/v"+versao+"/checksums.txt", somas.String())

	cache := t.TempDir()
	velhoBase, velhoCache, velhaVersao := baseReleases, dirCacheUsuario, Versao
	baseReleases = s.srv.URL
	dirCacheUsuario = func() (string, error) { return cache, nil }
	Versao = versao
	t.Setenv("GS_RELEASES_URL", "")
	t.Cleanup(func() { baseReleases, dirCacheUsuario, Versao = velhoBase, velhoCache, velhaVersao })
	return s, cache
}

func scriptTeste(t *testing.T) (string, []byte) {
	t.Helper()
	fonte := []byte("mostra \"salve do binario\"\n")
	arq := filepath.Join(t.TempDir(), "app.gs")
	if err := os.WriteFile(arq, fonte, 0644); err != nil {
		t.Fatal(err)
	}
	return arq, fonte
}

// confereEmbed checa o formato [base][fonte][len LE][magic].
func confereEmbed(t *testing.T, saida string, base, fonte []byte) {
	t.Helper()
	got, err := os.ReadFile(saida)
	if err != nil {
		t.Fatal(err)
	}
	var lenBuf [8]byte
	binary.LittleEndian.PutUint64(lenBuf[:], uint64(len(fonte)))
	esperado := append(append(append(append([]byte{}, base...), fonte...), lenBuf[:]...), buildMagic...)
	if !bytes.Equal(got, esperado) {
		t.Fatalf("binario gerado nao e base+fonte+rodape (%d bytes vs %d esperados)", len(got), len(esperado))
	}
}

func TestBuildAlvoBaixaReleaseConfereEUsaCache(t *testing.T) {
	alvo, maq := alvoEstrangeiro()
	goos, goarch, _ := strings.Cut(alvo, "/")
	base := elfFalso(maq)
	pacote := fmt.Sprintf("gs_9.9.9_%s_%s.tar.gz", goos, goarch)
	s, cache := releaseFalsa(t, "9.9.9", map[string][]byte{pacote: tarGz(t, "gs", base)})

	arq, fonte := scriptTeste(t)
	saida := filepath.Join(t.TempDir(), "app-linux")
	if _, err := construir(opcoesBuild{arquivo: arq, saida: saida, alvo: alvo}, io.Discard); err != nil {
		t.Fatalf("build --alvo falhou: %v", err)
	}
	confereEmbed(t, saida, base, fonte)
	if _, err := os.Stat(filepath.Join(cache, "gambiarrascript", "9.9.9", pacote)); err != nil {
		t.Fatalf("pacote nao foi pro cache: %v", err)
	}

	// segunda vez: cache conferido, zero rede
	antes := s.contaPedidos()
	var out bytes.Buffer
	if _, err := construir(opcoesBuild{arquivo: arq, saida: saida, alvo: alvo}, &out); err != nil {
		t.Fatal(err)
	}
	if s.contaPedidos() != antes || !strings.Contains(out.String(), "cache") {
		t.Fatalf("devia usar o cache sem ir na rede:\n%s", out.String())
	}

	// cache corrompido: rebaixa e confere de novo
	os.WriteFile(filepath.Join(cache, "gambiarrascript", "9.9.9", pacote), []byte("lixo"), 0644)
	if _, err := construir(opcoesBuild{arquivo: arq, saida: saida, alvo: alvo}, io.Discard); err != nil {
		t.Fatal(err)
	}
	if s.contaPedidos() == antes {
		t.Fatal("cache corrompido devia forcar download")
	}
	confereEmbed(t, saida, base, fonte)
}

func TestBuildAlvoRecusaChecksumErrado(t *testing.T) {
	alvo, maq := alvoEstrangeiro()
	goos, goarch, _ := strings.Cut(alvo, "/")
	pacote := fmt.Sprintf("gs_9.9.9_%s_%s.tar.gz", goos, goarch)
	s, cache := releaseFalsa(t, "9.9.9", map[string][]byte{pacote: tarGz(t, "gs", elfFalso(maq))})
	// alguem troca o pacote depois do checksums.txt publicado
	s.poe("/v9.9.9/"+pacote, string(tarGz(t, "gs", elfFalso(maq)[:70])))

	arq, _ := scriptTeste(t)
	saida := filepath.Join(t.TempDir(), "app")
	_, err := construir(opcoesBuild{arquivo: arq, saida: saida, alvo: alvo}, io.Discard)
	if err == nil || !strings.Contains(err.Error(), "RECUSADO") {
		t.Fatalf("checksum errado devia ser recusado, veio %v", err)
	}
	if _, err := os.Stat(saida); err == nil {
		t.Fatal("nao devia gerar binario")
	}
	if _, err := os.Stat(filepath.Join(cache, "gambiarrascript", "9.9.9", pacote)); err == nil {
		t.Fatal("pacote adulterado nao devia ir pro cache")
	}
}

func TestBuildAlvoWindowsZipGanhaExe(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("windows/amd64 e o alvo nativo aqui")
	}
	base := peFalso()
	releaseFalsa(t, "9.9.9", map[string][]byte{"gs_9.9.9_windows_amd64.zip": zipCom(t, "gs.exe", base)})
	arq, fonte := scriptTeste(t)

	dir := t.TempDir()
	wd, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(wd)
	saida, err := construir(opcoesBuild{arquivo: arq, alvo: "windows/amd64"}, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	if saida != "app.exe" {
		t.Fatalf("alvo windows devia gerar app.exe, gerou %q", saida)
	}
	confereEmbed(t, filepath.Join(dir, saida), base, fonte)
}

func TestBuildAlvoDevSemReleaseSugereGsBase(t *testing.T) {
	alvo, _ := alvoEstrangeiro()
	s, _ := releaseFalsa(t, "9.9.9", map[string][]byte{})
	arq, _ := scriptTeste(t)
	for _, v := range []string{"dev", "0.4.0-dirty+abc", ""} {
		Versao = v
		_, err := construir(opcoesBuild{arquivo: arq, saida: filepath.Join(t.TempDir(), "x"), alvo: alvo}, io.Discard)
		if err == nil || !strings.Contains(err.Error(), "--gs-base") {
			t.Fatalf("versao %q: devia sugerir --gs-base, veio %v", v, err)
		}
	}
	if s.contaPedidos() != 0 {
		t.Fatal("versao de dev nem devia ir na rede")
	}
	// versao com cara de release mas sem release publicada (404)
	Versao = "9.9.8"
	_, err := construir(opcoesBuild{arquivo: arq, saida: filepath.Join(t.TempDir(), "x"), alvo: alvo}, io.Discard)
	if err == nil || !strings.Contains(err.Error(), "--gs-base") || !strings.Contains(err.Error(), "checksums.txt") {
		t.Fatalf("release inexistente devia sugerir --gs-base, veio %v", err)
	}
}

func TestBuildGsBaseOffline(t *testing.T) {
	alvo, maq := alvoEstrangeiro()
	s, _ := releaseFalsa(t, "9.9.9", map[string][]byte{})
	Versao = "dev"
	base := elfFalso(maq)
	caminhoBase := filepath.Join(t.TempDir(), "gs-alvo")
	os.WriteFile(caminhoBase, base, 0755)

	arq, fonte := scriptTeste(t)
	saida := filepath.Join(t.TempDir(), "app")
	if _, err := construir(opcoesBuild{arquivo: arq, saida: saida, alvo: alvo, gsBase: caminhoBase}, io.Discard); err != nil {
		t.Fatalf("--gs-base falhou: %v", err)
	}
	confereEmbed(t, saida, base, fonte)
	if s.contaPedidos() != 0 {
		t.Fatal("--gs-base nao devia ir na rede")
	}

	// base de outra arquitetura e recusada
	outra := elf.EM_X86_64
	if maq == elf.EM_X86_64 {
		outra = elf.EM_AARCH64
	}
	os.WriteFile(caminhoBase, elfFalso(outra), 0755)
	_, err := construir(opcoesBuild{arquivo: arq, saida: saida, alvo: alvo, gsBase: caminhoBase}, io.Discard)
	if err == nil || !strings.Contains(err.Error(), "nao e um executavel de "+alvo) {
		t.Fatalf("base de arquitetura errada devia ser recusada, veio %v", err)
	}
	// e um script qualquer no lugar do binario tambem
	os.WriteFile(caminhoBase, []byte("#!/bin/sh\necho oi\n"), 0755)
	if _, err := construir(opcoesBuild{arquivo: arq, saida: saida, alvo: alvo, gsBase: caminhoBase}, io.Discard); err == nil {
		t.Fatal("base que nao e ELF devia ser recusada")
	}
}

func TestBuildAlvoInvalido(t *testing.T) {
	arq, _ := scriptTeste(t)
	for _, alvo := range []string{"linux", "plan9/amd64", "linux/"} {
		if _, err := construir(opcoesBuild{arquivo: arq, saida: filepath.Join(t.TempDir(), "x"), alvo: alvo}, io.Discard); err == nil {
			t.Fatalf("alvo %q devia ser recusado", alvo)
		}
	}
}

func TestParseArgsBuild(t *testing.T) {
	o, err := parseArgsBuild([]string{"app.gs", "--alvo", "linux/amd64", "-o", "saida", "--gs-base=/tmp/gs"})
	if err != nil || o.arquivo != "app.gs" || o.alvo != "linux/amd64" || o.saida != "saida" || o.gsBase != "/tmp/gs" {
		t.Fatalf("parse errado: %+v %v", o, err)
	}
	o, err = parseArgsBuild([]string{"--alvo=windows/amd64", "app.gs"})
	if err != nil || o.alvo != "windows/amd64" || o.arquivo != "app.gs" {
		t.Fatalf("parse errado: %+v %v", o, err)
	}
	for _, ruim := range [][]string{{}, {"app.gs", "--alvo"}, {"app.gs", "--xpto"}, {"a.gs", "b.gs"}} {
		if _, err := parseArgsBuild(ruim); err == nil {
			t.Fatalf("%v devia dar erro", ruim)
		}
	}
}

func TestProcuraChecksum(t *testing.T) {
	somas := []byte(strings.Repeat("a", 64) + "  gs_1.0.0_linux_amd64.tar.gz\n" +
		strings.Repeat("B", 64) + " *gs_1.0.0_windows_amd64.zip\n")
	if h, ok := procuraChecksum(somas, "gs_1.0.0_linux_amd64.tar.gz"); !ok || h != strings.Repeat("a", 64) {
		t.Fatalf("nao achou o linux: %q", h)
	}
	if h, ok := procuraChecksum(somas, "gs_1.0.0_windows_amd64.zip"); !ok || h != strings.Repeat("b", 64) {
		t.Fatalf("nao achou o windows (formato binario do sha256sum): %q", h)
	}
	if _, ok := procuraChecksum(somas, "gs_1.0.0_linux_arm64.tar.gz"); ok {
		t.Fatal("achou o que nao existe")
	}
}
