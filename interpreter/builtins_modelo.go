package interpreter

import (
	"bytes"
	"errors"
	"path/filepath"

	"gambiarrascript/modelo"
	"gambiarrascript/object"
)

// Templates: renderiza(texto, dados, [opcoes]) e renderiza_arquivo(caminho,
// dados, [opcoes]). O motor mora no pacote modelo; aqui so a cola com a
// linguagem (pra_json e formata entram como funcao). Funcionam no navegador
// tambem (renderiza_arquivo so acha arquivo se tiver sistema de arquivos).

// opcoesModelo le {"pasta": texto, "estrito": booleano, "escapa": booleano}.
func opcoesModelo(nome string, args []object.Object, pos int) (modelo.Opcoes, *object.Erro) {
	op := modelo.Opcoes{
		Json: func(v object.Object) (string, error) {
			var buf bytes.Buffer
			if e := escreveJson(&buf, v); e != nil {
				return "", errors.New(e.Message)
			}
			return buf.String(), nil
		},
		Formata: func(f string, v object.Object) (string, error) {
			r := builtinFormata([]object.Object{&object.Texto{Value: f}, v})
			if e, ok := r.(*object.Erro); ok {
				return "", errors.New(e.Message)
			}
			return r.Inspect(), nil
		},
	}
	if len(args) <= pos {
		return op, nil
	}
	d, ok := args[pos].(*object.Dicionario)
	if !ok {
		return op, erroBuiltin("%s(): as opcoes tem que ser dicionario ({\"pasta\", \"estrito\", \"escapa\"}), veio %s", nome, object.NomeTipo(args[pos]))
	}
	for _, par := range d.Pares() {
		chave, _ := par.Chave.(*object.Texto)
		if chave == nil {
			return op, erroBuiltin("%s(): opcao com chave que nao e texto: %s", nome, par.Chave.Inspect())
		}
		switch chave.Value {
		case "pasta":
			t, ok := par.Valor.(*object.Texto)
			if !ok {
				return op, erroBuiltin("%s(): a opcao \"pasta\" tem que ser texto", nome)
			}
			op.Pasta = t.Value
		case "estrito", "escapa":
			b, ok := par.Valor.(*object.Booleano)
			if !ok {
				return op, erroBuiltin("%s(): a opcao %q tem que ser deu_bom ou deu_ruim", nome, chave.Value)
			}
			if chave.Value == "estrito" {
				op.Estrito = b.Value
			} else {
				op.SemEscapar = !b.Value
			}
		default:
			return op, erroBuiltin("%s(): opcao %q nao existe (tem: pasta, estrito, escapa)", nome, chave.Value)
		}
	}
	return op, nil
}

func dadosModelo(nome string, args []object.Object) (object.Object, *object.Erro) {
	if len(args) < 2 {
		return &object.Nada{}, nil
	}
	switch args[1].(type) {
	case *object.Dicionario, *object.Instancia, *object.Nada:
		return args[1], nil
	}
	return nil, erroBuiltin("%s(): os dados tem que ser dicionario (ou treta), veio %s", nome, object.NomeTipo(args[1]))
}

func erroDeModelo(err error) *object.Erro {
	var em *modelo.Erro
	if errors.As(err, &em) && em.IO {
		return erroBuiltinKind(KindIO, "%s", err.Error())
	}
	return erroBuiltin("%s", err.Error())
}

// builtinRenderiza: renderiza(modelo, [dados], [opcoes]) → texto.
func builtinRenderiza(args []object.Object) object.Object {
	if len(args) < 1 || len(args) > 3 {
		return erroBuiltin("renderiza() quer 1 a 3 argumentos (modelo, [dados], [opcoes]), veio %d", len(args))
	}
	fonte, ok := args[0].(*object.Texto)
	if !ok {
		return erroBuiltin("renderiza(): o modelo tem que ser texto, veio %s (pra arquivo use renderiza_arquivo)", object.NomeTipo(args[0]))
	}
	dados, e := dadosModelo("renderiza", args)
	if e != nil {
		return e
	}
	op, e := opcoesModelo("renderiza", args, 2)
	if e != nil {
		return e
	}
	m, err := modelo.Compila("(texto)", fonte.Value)
	if err != nil {
		return erroDeModelo(err)
	}
	out, err := m.Renderiza(dados, op)
	if err != nil {
		return erroDeModelo(err)
	}
	return &object.Texto{Value: out}
}

// builtinRenderizaArquivo: renderiza_arquivo(caminho, [dados], [opcoes]) →
// texto. O modelo compilado fica em cache (caminho + mtime).
func builtinRenderizaArquivo(args []object.Object) object.Object {
	if len(args) < 1 || len(args) > 3 {
		return erroBuiltin("renderiza_arquivo() quer 1 a 3 argumentos (caminho, [dados], [opcoes]), veio %d", len(args))
	}
	caminho, ok := args[0].(*object.Texto)
	if !ok {
		return erroBuiltin("renderiza_arquivo(): o caminho tem que ser texto, veio %s", object.NomeTipo(args[0]))
	}
	dados, e := dadosModelo("renderiza_arquivo", args)
	if e != nil {
		return e
	}
	op, e := opcoesModelo("renderiza_arquivo", args, 2)
	if e != nil {
		return e
	}
	// nome nos erros: relativo a pasta dos modelos (ou so o nome do arquivo)
	nome := filepath.Base(caminho.Value)
	if op.Pasta != "" {
		if raiz, err := filepath.Abs(op.Pasta); err == nil {
			if abs, err := filepath.Abs(caminho.Value); err == nil {
				if rel, err := filepath.Rel(raiz, abs); err == nil && !filepath.IsAbs(rel) && rel != ".." && (len(rel) < 3 || rel[:3] != ".."+string(filepath.Separator)) {
					nome = filepath.ToSlash(rel)
				}
			}
		}
	}
	m, err := modelo.CarregaArquivo(caminho.Value, nome)
	if err != nil {
		return erroDeModelo(err)
	}
	out, err := m.Renderiza(dados, op)
	if err != nil {
		return erroDeModelo(err)
	}
	return &object.Texto{Value: out}
}
