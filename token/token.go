package token

type TokenType string

type Token struct {
	Type    TokenType
	Literal string
	Line    int
	Coluna  int
}

const (
	ILLEGAL = "ILLEGAL"
	EOF     = "EOF"

	IDENT  = "IDENT"
	NUMERO = "NUMERO"
	TEXTO  = "TEXTO"

	ASSIGN  = "="
	PLUS    = "+"
	MINUS   = "-"
	STAR    = "*"
	SLASH   = "/"
	PERCENT = "%"

	// atribuicao composta aritmetica
	PLUSASSIGN    = "+="
	MINUSASSIGN   = "-="
	STARASSIGN    = "*="
	SLASHASSIGN   = "/="
	PERCENTASSIGN = "%="

	EQ  = "=="
	NEQ = "!="
	LT  = "<"
	GT  = ">"
	LTE = "<="
	GTE = ">="

	// bitwise
	BAND         = "&"  // and bitwise
	BOR          = "|"  // or bitwise
	BXOR         = "^"  // xor bitwise
	BNOT         = "~"  // not bitwise (prefixo)
	LSHIFT       = "<<" // shift esquerda
	RSHIFT       = ">>" // shift direita
	BANDASSIGN   = "&="
	BORASSIGN    = "|="
	BXORASSIGN   = "^="
	LSHIFTASSIGN = "<<="
	RSHIFTASSIGN = ">>="

	COMMA    = ","
	LPAREN   = "("
	RPAREN   = ")"
	LBRACKET = "["
	RBRACKET = "]"
	LBRACE   = "{"
	RBRACE   = "}"
	COLON    = ":"

	RANGE    = ".."
	DOT      = "."  // acesso por ponto: obj.campo
	QDOT     = "?." // navegacao segura: obj?.campo
	COALESCE = "??" // coalescing: x ?? padrao
	ELLIPSIS = "..." // varargs: gambiarra f(primeiro, ...resto)

	ENTAO = "ENTAO" // se_colar cond ENTAO a se_nao_colar b (ternario)
	COMO  = "COMO"  // importa "x.gs" COMO alias

	BOTA         = "BOTA"
	MOSTRA       = "MOSTRA"
	SE_COLAR     = "SE_COLAR"
	SE_NAO_COLAR = "SE_NAO_COLAR"
	ENQUANTO     = "ENQUANTO"
	PRA_CADA     = "PRA_CADA"
	DE           = "DE"
	ATE          = "ATE"
	EM           = "EM"
	GAMBIARRA    = "GAMBIARRA"
	FUNCIONA     = "FUNCIONA"
	ARRUMA       = "ARRUMA"
	QUEBROU      = "QUEBROU"
	VAZA         = "VAZA"
	CONTINUA     = "CONTINUA"
	DEU_BOM      = "DEU_BOM"
	DEU_RUIM     = "DEU_RUIM"
	NADA         = "NADA"
	ACABOU       = "ACABOU_FINALMENTE"
	FINALMENTE   = "FINALMENTE"
	ESCOLHE      = "ESCOLHE" // switch
	CASO         = "CASO"    // case
	E            = "E"
	OU           = "OU"
	NAO          = "NAO"
	IMPORTA      = "IMPORTA"

	// concorrencia
	BORA = "BORA" // bora fn(args) -> Futuro
)

var keywords = map[string]TokenType{
	"bota":              BOTA,
	"mostra":            MOSTRA,
	"se_colar":          SE_COLAR,
	"se_nao_colar":      SE_NAO_COLAR,
	"enquanto":          ENQUANTO,
	"pra_cada":          PRA_CADA,
	"de":                DE,
	"ate":               ATE,
	"em":                EM,
	"gambiarra":         GAMBIARRA,
	"funciona":          FUNCIONA,
	"arruma":            ARRUMA,
	"quebrou":           QUEBROU,
	"vaza":              VAZA,
	"continua":          CONTINUA,
	"deu_bom":           DEU_BOM,
	"deu_ruim":          DEU_RUIM,
	"nada":              NADA,
	"acabou_finalmente": ACABOU,
	"finalmente":        FINALMENTE,
	"escolhe":           ESCOLHE,
	"caso":              CASO,
	"e":                 E,
	"ou":                OU,
	"nao":               NAO,
	"importa":           IMPORTA,
	"bora":              BORA,
	"entao":             ENTAO,
	"como":              COMO,
}

// LookupIdent devolve o TokenType de uma keyword, ou IDENT se for um nome comum.
func LookupIdent(ident string) TokenType {
	if tok, ok := keywords[ident]; ok {
		return tok
	}
	return IDENT
}
