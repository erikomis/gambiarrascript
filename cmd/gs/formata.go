package main

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// coletaArquivosGs expande os caminhos dados: um arquivo entra direto; um
// diretorio e varrido recursivamente coletando todos os *.gs (na ordem lexical
// estavel do WalkDir). Usado pelo `gs formata` pra aceitar diretorios.
func coletaArquivosGs(caminhos []string) ([]string, error) {
	var out []string
	for _, c := range caminhos {
		info, err := os.Stat(c)
		if err != nil {
			return nil, err
		}
		if !info.IsDir() {
			out = append(out, c)
			continue
		}
		err = filepath.WalkDir(c, func(p string, d fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if !d.IsDir() && strings.HasSuffix(p, ".gs") {
				out = append(out, p)
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	return out, nil
}
