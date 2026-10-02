package object

import "math"

// Predefinidas sao os VALORES (nao gambiarras) que todo programa ja enxerga
// sem declarar, tipo `pi`. Tree-walker, VM, linter e autocomplete leem daqui.
// Como builtin, um `bota pi = 3` do usuario sombreia o predefinido.
var Predefinidas = map[string]Object{
	"pi": NumFloat(math.Pi),
}
