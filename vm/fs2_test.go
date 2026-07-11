package vm

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// TestParidadeFs2 garante que os builtins de fs parte 2 (read-only, entao
// idempotentes pra rodar nos dois engines) resolvem e rodam identico na VM.
// copia/move tem efeito colateral (nao da pra rodar 2x), mas usam o mesmo
// caminho de registro dos builtins puros, coberto por estes.
func TestParidadeFs2(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "x.txt")
	if err := os.WriteFile(f, []byte("12345"), 0o644); err != nil {
		t.Fatal(err)
	}
	comparaEngines(t, fmt.Sprintf(`mostra tamanho_arquivo(%q)`, f))
	comparaEngines(t, fmt.Sprintf(`mostra tamanho(glob(%q))`, filepath.Join(dir, "*.txt")))
	comparaEngines(t, fmt.Sprintf(`mostra modificado_em(%q)`, f))
}
