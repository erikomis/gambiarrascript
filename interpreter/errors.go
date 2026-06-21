package interpreter

import (
	"fmt"

	"gambiarrascript/object"
)

func newError(linha int, formato string, args ...interface{}) *object.Erro {
	msg := fmt.Sprintf(formato, args...)
	return &object.Erro{Message: fmt.Sprintf("deu ruim na linha %d: %s", linha, msg)}
}

func isError(obj object.Object) bool {
	return obj != nil && obj.Type() == object.ERRO_OBJ
}
