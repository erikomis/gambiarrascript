package modelo

import (
	"os"
	"path/filepath"
	"sync"
	"time"
)

// cache dos modelos de arquivo ja compilados: chave = caminho absoluto + nome
// de exibicao; vale enquanto o mtime e o tamanho do arquivo nao mudam (editou
// o .html com o servidor rodando, a proxima requisicao ja pega o novo).
type entradaCache struct {
	mod     time.Time
	tamanho int64
	m       *Modelo
}

var (
	cacheMu sync.Mutex
	cache   = map[string]entradaCache{}
)

// CarregaArquivo le e compila o modelo do arquivo (com cache). nome e o que
// aparece nos erros ("" = o nome do arquivo).
func CarregaArquivo(caminho, nome string) (*Modelo, error) {
	abs, err := filepath.Abs(caminho)
	if err != nil {
		abs = caminho
	}
	if nome == "" {
		nome = filepath.Base(abs)
	}
	info, err := os.Stat(abs)
	if err != nil {
		return nil, &Erro{Modelo: nome, Msg: "nao achei o arquivo " + caminho, IO: true}
	}
	if info.IsDir() {
		return nil, &Erro{Modelo: nome, Msg: caminho + " e uma pasta, nao um arquivo", IO: true}
	}
	chave := abs + "\x00" + nome
	cacheMu.Lock()
	e, ok := cache[chave]
	cacheMu.Unlock()
	if ok && e.mod.Equal(info.ModTime()) && e.tamanho == info.Size() {
		return e.m, nil
	}
	fonte, err := os.ReadFile(abs)
	if err != nil {
		return nil, &Erro{Modelo: nome, Msg: "nao consegui ler " + caminho + ": " + err.Error(), IO: true}
	}
	m, err := Compila(nome, string(fonte))
	if err != nil {
		return nil, err
	}
	m.Caminho = abs
	cacheMu.Lock()
	cache[chave] = entradaCache{mod: info.ModTime(), tamanho: info.Size(), m: m}
	cacheMu.Unlock()
	return m, nil
}
