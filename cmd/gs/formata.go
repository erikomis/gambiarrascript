package main

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"gambiarrascript/formatter"
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

// funcFormata e quem formata a fonte (o teste troca por um com bug).
type funcFormata func(fonte string) (saida string, errsParse []string)

// errParse sao os erros de parse da fonte (o CLI lista um por linha).
type errParse []string

func (e errParse) Error() string { return strings.Join(e, "; ") }

// formataConferido formata e passa pela trava (formatter.Confere): se a
// saida nao parsear, mudar o AST ou perder comentario, e erro e nada deve
// ser escrito.
func formataConferido(fonte string, formata funcFormata) (string, error) {
	saida, errs := formata(fonte)
	if len(errs) != 0 {
		return "", errParse(errs)
	}
	// preserva a quebra de linha final do original
	if strings.HasSuffix(fonte, "\n") && !strings.HasSuffix(saida, "\n") {
		saida += "\n"
	}
	if err := formatter.Confere(fonte, saida); err != nil {
		return "", err
	}
	return saida, nil
}

// escreveFormatado sobrescreve o arquivo com a versao formatada, so se mudou
// e so se passou na trava. Devolve se escreveu.
func escreveFormatado(caminho string, formata funcFormata) (bool, error) {
	fonte, err := os.ReadFile(caminho)
	if err != nil {
		return false, err
	}
	saida, err := formataConferido(string(fonte), formata)
	if err != nil {
		return false, err
	}
	if saida == string(fonte) {
		return false, nil
	}
	return true, os.WriteFile(caminho, []byte(saida), 0o644)
}
