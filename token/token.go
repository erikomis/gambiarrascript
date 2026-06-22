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

	EQ  = "=="
	NEQ = "!="
	LT  = "<"
	GT  = ">"
	LTE = "<="
	GTE = ">="

	COMMA    = ","
	LPAREN   = "("
	RPAREN   = ")"
	LBRACKET = "["
	RBRACKET = "]"
	LBRACE   = "{"
	RBRACE   = "}"
	COLON    = ":"

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
	E            = "E"
	OU           = "OU"
	NAO          = "NAO"
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
	"e":                 E,
	"ou":                OU,
	"nao":               NAO,
}

// LookupIdent devolve o TokenType de uma keyword, ou IDENT se for um nome comum.
func LookupIdent(ident string) TokenType {
	if tok, ok := keywords[ident]; ok {
		return tok
	}
	return IDENT
}
