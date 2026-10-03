package interpreter

import (
	"bytes"
	"errors"
	"testing"

	"gambiarrascript/object"
)

// conexaoFalsa e uma object.Conexao em memoria: o que entra por Envia sai
// por Recebe, e depois de Fecha o Recebe devolve nil (= nada).
type conexaoFalsa struct {
	fila    []object.Object
	fechada int
	quebra  bool
}

func (c *conexaoFalsa) Envia(v object.Object) error {
	if c.quebra {
		return errors.New("conexao caiu")
	}
	c.fila = append(c.fila, v)
	return nil
}

func (c *conexaoFalsa) Recebe() (object.Object, error) {
	if len(c.fila) == 0 {
		return nil, nil
	}
	v := c.fila[0]
	c.fila = c.fila[1:]
	return v, nil
}

func (c *conexaoFalsa) Fecha() error {
	c.fechada++
	return nil
}

func TestEnviaRecebeFechaDespachamPraConexao(t *testing.T) {
	i := New(&bytes.Buffer{})
	falsa := &conexaoFalsa{}
	con := &object.Nativo{Rotulo: "teste", Valor: falsa}

	if r := i.builtinEnvia([]object.Object{con, &object.Texto{Value: "oi"}}); r != NADA {
		t.Fatalf("envia devolveu %s", r.Inspect())
	}
	if r := i.builtinRecebe([]object.Object{con}); r.Inspect() != "oi" {
		t.Fatalf("recebe devolveu %s", r.Inspect())
	}
	// fila vazia = outro lado fechou: nada, igual cano fechado
	if r := i.builtinRecebe([]object.Object{con}); r != NADA {
		t.Fatalf("recebe sem mensagem devia ser nada, veio %s", r.Inspect())
	}
	if r := i.builtinFecha([]object.Object{con}); r != NADA || falsa.fechada != 1 {
		t.Fatalf("fecha nao chegou na conexao: %s, fechada=%d", r.Inspect(), falsa.fechada)
	}

	falsa.quebra = true
	r := i.builtinEnvia([]object.Object{con, &object.Texto{Value: "x"}})
	e, ok := r.(*object.Erro)
	if !ok || e.Kind != KindRede {
		t.Fatalf("erro da conexao devia virar erro de rede, veio %s", r.Inspect())
	}
}
