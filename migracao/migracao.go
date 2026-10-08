// Package migracao aplica e reverte migracoes de banco escritas em SQL puro.
//
// Cada migracao e um par de arquivos numa pasta (por padrao `migracoes/`):
//
//	001_cria_usuarios.sobe.sql    # obrigatorio: o que aplica
//	001_cria_usuarios.desce.sql   # opcional: o que desfaz (gs migra desce)
//
// O numero (qualquer quantidade de digitos) e a versao; a ordem de aplicacao
// e a numerica. O que ja rodou fica anotado na tabela gs_migracoes, com o
// sha256 do .sobe.sql: se um arquivo ja aplicado mudar, nada roda ate alguem
// resolver (migracao aplicada nao se edita — cria outra).
//
// Usado pelo `gs migra` (cmd/gs) e pela builtin migra(conexao, [pasta]).
package migracao

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"
)

// PastaPadrao e onde as migracoes moram quando ninguem diz outra coisa.
const PastaPadrao = "migracoes"

// Tabela e o nome da tabela de controle.
const Tabela = "gs_migracoes"

// Migracao e um par de arquivos lido da pasta.
type Migracao struct {
	Versao   int64
	Nome     string // a descricao depois do numero ("cria_usuarios")
	Sobe     string // conteudo do .sobe.sql
	Desce    string // conteudo do .desce.sql ("" se nao tem)
	TemDesce bool
	Checksum string // sha256 hex do .sobe.sql
}

// Rotulo e o jeito de mostrar a migracao: "001_cria_usuarios".
func (m Migracao) Rotulo() string { return rotulo(m.Versao, m.Nome) }

func rotulo(v int64, nome string) string { return fmt.Sprintf("%03d_%s", v, nome) }

// Aplicada e uma linha da tabela gs_migracoes.
type Aplicada struct {
	Versao     int64
	Nome       string
	Checksum   string
	AplicadaEm string
}

// Estado junta arquivo e banco pro `status`.
type Estado struct {
	Versao     int64
	Nome       string
	Aplicada   bool
	AplicadaEm string
	Alterada   bool // aplicada, mas o .sobe.sql mudou depois
	Sumiu      bool // aplicada, mas o arquivo nao esta mais na pasta
}

var reArquivo = regexp.MustCompile(`^(\d+)_([A-Za-z0-9_\-]+)\.(sobe|desce)\.sql$`)

// Carrega le a pasta e devolve as migracoes em ordem de versao. Arquivo .sql
// fora do padrao e erro (melhor do que ignorar calado uma migracao com nome
// errado); outros arquivos (README, .gitkeep) sao ignorados.
func Carrega(pasta string) ([]Migracao, error) {
	entradas, err := os.ReadDir(pasta)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("pasta de migracoes %q nao existe (cria com: gs migra novo <nome>)", pasta)
		}
		return nil, fmt.Errorf("nao consegui ler a pasta %q: %v", pasta, err)
	}
	porVersao := map[int64]*Migracao{}
	for _, e := range entradas {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".sql") {
			continue
		}
		p := reArquivo.FindStringSubmatch(e.Name())
		if p == nil {
			return nil, fmt.Errorf("arquivo %q fora do padrao NNN_descricao.sobe.sql / NNN_descricao.desce.sql", e.Name())
		}
		v, err := strconv.ParseInt(p[1], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("versao invalida em %q: %v", e.Name(), err)
		}
		conteudo, err := os.ReadFile(filepath.Join(pasta, e.Name()))
		if err != nil {
			return nil, fmt.Errorf("nao consegui ler %q: %v", e.Name(), err)
		}
		m := porVersao[v]
		if m == nil {
			m = &Migracao{Versao: v, Nome: p[2]}
			porVersao[v] = m
		} else if m.Nome != p[2] {
			return nil, fmt.Errorf("versao %d repetida: %q e %q", v, rotulo(v, m.Nome), rotulo(v, p[2]))
		}
		if p[3] == "sobe" {
			if m.Checksum != "" {
				return nil, fmt.Errorf("versao %d repetida (dois .sobe.sql)", v)
			}
			m.Sobe = string(conteudo)
			m.Checksum = Checksum(conteudo)
		} else {
			m.Desce = string(conteudo)
			m.TemDesce = true
		}
	}
	out := make([]Migracao, 0, len(porVersao))
	for _, m := range porVersao {
		if m.Checksum == "" {
			return nil, fmt.Errorf("%s.desce.sql sem o %s.sobe.sql", m.Rotulo(), m.Rotulo())
		}
		out = append(out, *m)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Versao < out[j].Versao })
	return out, nil
}

// Checksum e o sha256 hex do conteudo.
func Checksum(b []byte) string {
	s := sha256.Sum256(b)
	return hex.EncodeToString(s[:])
}

// Migrador roda as migracoes de uma pasta num banco. Driver e o nome do
// driver database/sql ("sqlite", "mysql", "pgx") — decide placeholder,
// transacao e como mandar arquivo com varios comandos.
type Migrador struct {
	DB     *sql.DB
	Driver string
	Pasta  string
}

// Novo monta o migrador (pasta vazia = PastaPadrao).
func Novo(db *sql.DB, driver, pasta string) *Migrador {
	if pasta == "" {
		pasta = PastaPadrao
	}
	return &Migrador{DB: db, Driver: driver, Pasta: pasta}
}

// ddlTransacional: postgres e sqlite desfazem CREATE/ALTER no rollback.
// MySQL/MariaDB fazem commit implicito a cada DDL, entao la cada comando roda
// solto (falhou no meio = o que ja rodou fica).
func (m *Migrador) ddlTransacional() bool { return m.Driver != "mysql" }

func (m *Migrador) ph(i int) string {
	if m.Driver == "pgx" {
		return "$" + strconv.Itoa(i)
	}
	return "?"
}

func (m *Migrador) criaTabela() error {
	_, err := m.DB.Exec(`CREATE TABLE IF NOT EXISTS ` + Tabela + ` (
	versao BIGINT PRIMARY KEY,
	nome VARCHAR(255) NOT NULL,
	checksum VARCHAR(64) NOT NULL,
	aplicada_em VARCHAR(40) NOT NULL
)`)
	if err != nil {
		return fmt.Errorf("nao consegui criar a tabela %s: %v", Tabela, err)
	}
	return nil
}

// Aplicadas le a tabela de controle (cria se nao existe), em ordem de versao.
func (m *Migrador) Aplicadas() ([]Aplicada, error) {
	if err := m.criaTabela(); err != nil {
		return nil, err
	}
	rows, err := m.DB.Query(`SELECT versao, nome, checksum, aplicada_em FROM ` + Tabela + ` ORDER BY versao`)
	if err != nil {
		return nil, fmt.Errorf("nao consegui ler %s: %v", Tabela, err)
	}
	defer rows.Close()
	var out []Aplicada
	for rows.Next() {
		var a Aplicada
		if err := rows.Scan(&a.Versao, &a.Nome, &a.Checksum, &a.AplicadaEm); err != nil {
			return nil, fmt.Errorf("lendo %s: %v", Tabela, err)
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// carregaEConfere le pasta + banco e recusa seguir se algum arquivo aplicado
// mudou ou sumiu.
func (m *Migrador) carregaEConfere() ([]Migracao, []Aplicada, error) {
	migs, err := Carrega(m.Pasta)
	if err != nil {
		return nil, nil, err
	}
	aplicadas, err := m.Aplicadas()
	if err != nil {
		return nil, nil, err
	}
	porVersao := map[int64]Migracao{}
	for _, mg := range migs {
		porVersao[mg.Versao] = mg
	}
	for _, a := range aplicadas {
		mg, ok := porVersao[a.Versao]
		if !ok {
			return nil, nil, fmt.Errorf("a migracao %s ja foi aplicada mas o arquivo .sobe.sql sumiu da pasta %q", rotulo(a.Versao, a.Nome), m.Pasta)
		}
		if mg.Checksum != a.Checksum {
			return nil, nil, fmt.Errorf("a migracao %s ja foi aplicada mas o arquivo mudou depois (checksum era %s..., agora %s...). "+
				"Migracao aplicada nao se edita: desfaz a mudanca e cria uma nova (gs migra novo <nome>)",
				mg.Rotulo(), a.Checksum[:min(12, len(a.Checksum))], mg.Checksum[:12])
		}
	}
	return migs, aplicadas, nil
}

// Sobe aplica tudo que esta pendente, em ordem. Devolve o que aplicou. Cada
// migracao roda numa transacao (onde o banco desfaz DDL); se uma falha, para
// ali e as anteriores ficam.
func (m *Migrador) Sobe() ([]Migracao, error) {
	migs, aplicadas, err := m.carregaEConfere()
	if err != nil {
		return nil, err
	}
	feitas := map[int64]bool{}
	for _, a := range aplicadas {
		feitas[a.Versao] = true
	}
	var out []Migracao
	for _, mg := range migs {
		if feitas[mg.Versao] {
			continue
		}
		reg := `INSERT INTO ` + Tabela + ` (versao, nome, checksum, aplicada_em) VALUES (` +
			m.ph(1) + `, ` + m.ph(2) + `, ` + m.ph(3) + `, ` + m.ph(4) + `)`
		agora := time.Now().UTC().Format(time.RFC3339)
		if err := m.roda(mg.Sobe, reg, mg.Versao, mg.Nome, mg.Checksum, agora); err != nil {
			return out, fmt.Errorf("migracao %s falhou: %v", mg.Rotulo(), err)
		}
		out = append(out, mg)
	}
	return out, nil
}

// Desce reverte as n ultimas aplicadas (maior versao primeiro). Devolve o que
// reverteu. Precisa do .desce.sql de cada uma.
func (m *Migrador) Desce(n int) ([]Migracao, error) {
	if n < 1 {
		return nil, fmt.Errorf("quantas migracoes descer? tem que ser 1 ou mais, veio %d", n)
	}
	migs, aplicadas, err := m.carregaEConfere()
	if err != nil {
		return nil, err
	}
	porVersao := map[int64]Migracao{}
	for _, mg := range migs {
		porVersao[mg.Versao] = mg
	}
	var out []Migracao
	for i := len(aplicadas) - 1; i >= 0 && len(out) < n; i-- {
		mg := porVersao[aplicadas[i].Versao]
		if !mg.TemDesce {
			return out, fmt.Errorf("a migracao %s nao tem %s.desce.sql — nao sei desfazer", mg.Rotulo(), mg.Rotulo())
		}
		apaga := `DELETE FROM ` + Tabela + ` WHERE versao = ` + m.ph(1)
		if err := m.roda(mg.Desce, apaga, mg.Versao); err != nil {
			return out, fmt.Errorf("desfazer %s falhou: %v", mg.Rotulo(), err)
		}
		out = append(out, mg)
	}
	return out, nil
}

// Status lista tudo (arquivos + o que so existe no banco) em ordem de versao.
// Nao recusa por checksum: marca Alterada/Sumiu pra pessoa ver.
func (m *Migrador) Status() ([]Estado, error) {
	migs, err := Carrega(m.Pasta)
	if err != nil {
		return nil, err
	}
	aplicadas, err := m.Aplicadas()
	if err != nil {
		return nil, err
	}
	porVersao := map[int64]*Estado{}
	var out []*Estado
	for _, mg := range migs {
		e := &Estado{Versao: mg.Versao, Nome: mg.Nome}
		porVersao[mg.Versao] = e
		out = append(out, e)
	}
	somaArquivo := map[int64]string{}
	for _, mg := range migs {
		somaArquivo[mg.Versao] = mg.Checksum
	}
	for _, a := range aplicadas {
		e := porVersao[a.Versao]
		if e == nil {
			e = &Estado{Versao: a.Versao, Nome: a.Nome, Sumiu: true}
			out = append(out, e)
		} else if somaArquivo[a.Versao] != a.Checksum {
			e.Alterada = true
		}
		e.Aplicada = true
		e.AplicadaEm = a.AplicadaEm
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Versao < out[j].Versao })
	res := make([]Estado, len(out))
	for i, e := range out {
		res[i] = *e
	}
	return res, nil
}

// roda executa o sql do arquivo e o registro na tabela de controle juntos:
// numa transacao quando o banco desfaz DDL, em sequencia quando nao.
func (m *Migrador) roda(script, registro string, args ...any) error {
	if !m.ddlTransacional() {
		for _, cmd := range Separa(script) {
			if _, err := m.DB.Exec(cmd); err != nil {
				return err
			}
		}
		_, err := m.DB.Exec(registro, args...)
		return err
	}
	tx, err := m.DB.Begin()
	if err != nil {
		return err
	}
	if strings.TrimSpace(script) != "" {
		// sqlite (modernc) e postgres (pgx, protocolo simples quando nao tem
		// parametro) aceitam varios comandos num Exec so — inclusive corpo de
		// trigger e funcao com ; dentro, que um separador ingenuo quebraria.
		if _, err := tx.Exec(script); err != nil {
			tx.Rollback()
			return err
		}
	}
	if _, err := tx.Exec(registro, args...); err != nil {
		tx.Rollback()
		return err
	}
	return tx.Commit()
}

// Separa quebra um script SQL nos `;` que estao fora de texto ('...', "...",
// `...`) e de comentario (-- , # e /* */). Pedaco vazio some. Usado no MySQL,
// que nao aceita varios comandos num Exec sem multiStatements.
func Separa(script string) []string {
	var out []string
	var atual strings.Builder
	r := []rune(script)
	fecha := func() {
		s := strings.TrimSpace(atual.String())
		if s != "" && !soComentario(s) {
			out = append(out, s)
		}
		atual.Reset()
	}
	for i := 0; i < len(r); i++ {
		c := r[i]
		switch {
		case c == '\'' || c == '"' || c == '`':
			atual.WriteRune(c)
			for i++; i < len(r); i++ {
				atual.WriteRune(r[i])
				if r[i] == '\\' && i+1 < len(r) {
					i++
					atual.WriteRune(r[i])
					continue
				}
				if r[i] == c {
					break
				}
			}
		case c == '-' && i+1 < len(r) && r[i+1] == '-', c == '#':
			for ; i < len(r) && r[i] != '\n'; i++ {
				atual.WriteRune(r[i])
			}
			if i < len(r) {
				atual.WriteRune('\n')
			}
		case c == '/' && i+1 < len(r) && r[i+1] == '*':
			atual.WriteString("/*")
			for i += 2; i < len(r); i++ {
				atual.WriteRune(r[i])
				if r[i] == '/' && r[i-1] == '*' {
					break
				}
			}
		case c == ';':
			fecha()
		default:
			atual.WriteRune(c)
		}
	}
	fecha()
	return out
}

// soComentario: o pedaco so tem comentario (sobra depois do ultimo `;`).
func soComentario(s string) bool {
	for _, linha := range strings.Split(s, "\n") {
		l := strings.TrimSpace(linha)
		if l == "" || strings.HasPrefix(l, "--") || strings.HasPrefix(l, "#") {
			continue
		}
		if strings.HasPrefix(l, "/*") && strings.HasSuffix(l, "*/") {
			continue
		}
		return false
	}
	return true
}

// CriaArquivos cria o proximo par NNN_nome.sobe.sql / NNN_nome.desce.sql na
// pasta (cria a pasta se precisar). Devolve os dois caminhos.
func CriaArquivos(pasta, nome string) (string, string, error) {
	if pasta == "" {
		pasta = PastaPadrao
	}
	limpo := LimpaNome(nome)
	if limpo == "" {
		return "", "", fmt.Errorf("nome da migracao vazio (ex: gs migra novo cria_usuarios)")
	}
	if err := os.MkdirAll(pasta, 0o755); err != nil {
		return "", "", fmt.Errorf("nao consegui criar a pasta %q: %v", pasta, err)
	}
	migs, err := Carrega(pasta)
	if err != nil {
		return "", "", err
	}
	var prox int64 = 1
	if len(migs) > 0 {
		prox = migs[len(migs)-1].Versao + 1
	}
	base := filepath.Join(pasta, rotulo(prox, limpo))
	sobe, desce := base+".sobe.sql", base+".desce.sql"
	if err := os.WriteFile(sobe, []byte("-- "+rotulo(prox, limpo)+": o que esta migracao faz\n\n"), 0o644); err != nil {
		return "", "", err
	}
	if err := os.WriteFile(desce, []byte("-- "+rotulo(prox, limpo)+": como desfazer o .sobe.sql\n\n"), 0o644); err != nil {
		return "", "", err
	}
	return sobe, desce, nil
}

// LimpaNome deixa o nome no formato do arquivo: minusculo, sem acento,
// espaco e pontuacao viram _.
func LimpaNome(nome string) string {
	var b strings.Builder
	sublinhado := false
	for _, c := range strings.ToLower(strings.TrimSpace(nome)) {
		c = tiraAcento(c)
		if c < unicode.MaxASCII && (unicode.IsLetter(c) || unicode.IsDigit(c)) {
			b.WriteRune(c)
			sublinhado = false
		} else if !sublinhado && b.Len() > 0 {
			b.WriteRune('_')
			sublinhado = true
		}
	}
	return strings.TrimRight(b.String(), "_")
}

func tiraAcento(c rune) rune {
	switch c {
	case 'á', 'à', 'â', 'ã', 'ä':
		return 'a'
	case 'é', 'è', 'ê', 'ë':
		return 'e'
	case 'í', 'ì', 'î', 'ï':
		return 'i'
	case 'ó', 'ò', 'ô', 'õ', 'ö':
		return 'o'
	case 'ú', 'ù', 'û', 'ü':
		return 'u'
	case 'ç':
		return 'c'
	}
	return c
}
