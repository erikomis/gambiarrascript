package main

import (
	"bytes"
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"

	"gambiarrascript/object"
)

// Script com importa: o build (aqui --alvo com --gs-base, offline) embute o
// pacote com as fontes dos modulos — inclusive o que veio de gs_modulos/ —
// e o binario acha os modulos sem os .gs no disco.
func TestBuildEmbuteModulos(t *testing.T) {
	alvo, maq := alvoEstrangeiro()
	releaseFalsa(t, "9.9.9", map[string][]byte{})
	Versao = "dev"
	base := elfFalso(maq)
	caminhoBase := filepath.Join(t.TempDir(), "gs-alvo")
	os.WriteFile(caminhoBase, base, 0755)

	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "lib"), 0755)
	os.MkdirAll(filepath.Join(dir, "gs_modulos"), 0755)
	os.WriteFile(filepath.Join(dir, "app.gs"), []byte("importa \"lib/util.gs\" como u\nmostra u.dobra(2)\n"), 0644)
	os.WriteFile(filepath.Join(dir, "lib", "util.gs"), []byte("importa \"datas.gs\"\ngambiarra dobra(n)\n    funciona n * 2\nacabou_finalmente\n"), 0644)
	os.WriteFile(filepath.Join(dir, "gs_modulos", "datas.gs"), []byte("bota hoje = 1\n"), 0644)

	saida := filepath.Join(t.TempDir(), "app")
	var out bytes.Buffer
	if _, err := construir(opcoesBuild{arquivo: filepath.Join(dir, "app.gs"), saida: saida, alvo: alvo, gsBase: caminhoBase}, &out); err != nil {
		t.Fatalf("build falhou: %v", err)
	}
	got, _ := os.ReadFile(saida)
	if !bytes.HasPrefix(got, base) || string(got[len(got)-len(buildMagicPacote):]) != buildMagicPacote {
		t.Fatalf("esperava base + pacote GSEMBED2\n%s", out.String())
	}
	tam := int(binary.LittleEndian.Uint64(got[len(got)-16 : len(got)-8]))
	payload := got[len(got)-16-tam : len(got)-16]

	velho := object.LeModulo
	t.Cleanup(func() { object.LeModulo = velho })
	fonte, principal, err := abrePacote(buildMagicPacote, payload)
	if err != nil || principal != filepath.Join(dir, "app.gs") || !bytes.Contains(fonte, []byte("lib/util.gs")) {
		t.Fatalf("pacote errado: principal=%q err=%v", principal, err)
	}
	// os modulos saem do pacote mesmo com os arquivos apagados
	os.RemoveAll(filepath.Join(dir, "lib"))
	os.RemoveAll(filepath.Join(dir, "gs_modulos"))
	for _, c := range []string{filepath.Join(dir, "lib", "util.gs"), filepath.Join(dir, "gs_modulos", "datas.gs")} {
		if _, err := object.LeModulo(c); err != nil {
			t.Fatalf("modulo %s nao foi embutido: %v", c, err)
		}
	}
}
