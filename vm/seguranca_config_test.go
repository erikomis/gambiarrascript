package vm

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"gambiarrascript/compiler"
	"gambiarrascript/interpreter"
	"gambiarrascript/lexer"
	"gambiarrascript/object"
	"gambiarrascript/parser"
)

// Libs de API de verdade (Tier 5b): senha, AES, JWT, token, log, flags, .env.
// Saida EXATA nos 2 engines — o que e aleatorio (hash, nonce, token) vira
// booleano/tamanho pra dar pra cravar.

func TestSegurancaNosDois(t *testing.T) {
	casos := []struct{ src, saida string }{
		// bcrypt: vetor conhecido + round-trip com custo baixo
		{`mostra confere_senha("U*U", "$2a$05$CCCCCCCCCCCCCCCCCCCCC.E5YPO9kmyuRGyh0XouQYb4YMJKvyOeW")
mostra confere_senha("U*V", "$2a$05$CCCCCCCCCCCCCCCCCCCCC.E5YPO9kmyuRGyh0XouQYb4YMJKvyOeW")
mostra confere_senha("x", "isso nem e hash")
bota h = hash_senha("hunter2", 4)
mostra tamanho(h)
mostra comeca_com(h, "$2a$04$")
mostra confere_senha("hunter2", h)
mostra confere_senha("Hunter2", h)`, "deu_bom\ndeu_ruim\ndeu_ruim\n60\ndeu_bom\ndeu_bom\ndeu_ruim\n"},
		// AES-256-GCM: chave crua (gera_chave e hex) e frase
		{`bota k = gera_chave()
mostra tamanho(k)
bota c1 = encripta("salve, tropa", k)
bota c2 = encripta("salve, tropa", k)
mostra c1 == c2
mostra decripta(c1, k)
bota kh = "000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f"
mostra decripta(encripta("", kh), kh) == ""
mostra decripta(encripta("com frase", "senha fraca"), "senha fraca")
arruma
    decripta(c1, kh)
quebrou err
    mostra erro_tipo(err)
    mostra erro_msg(err)
acabou_finalmente`, "44\ndeu_ruim\nsalve, tropa\ndeu_bom\ncom frase\nbuiltin\ndeu ruim: nao deu pra decriptar: chave errada ou texto adulterado\n"},
		// texto adulterado: troca um caractere do meio do base64
		{`bota k = gera_chave()
bota c = encripta("dinheiro", k)
bota meio = tamanho(c) / 2
bota troca = "A"
se_colar fatia(c, meio, meio + 1) == "A"
    bota troca = "B"
acabou_finalmente
bota ruim = fatia(c, 0, meio) + troca + fatia(c, meio + 1)
arruma
    decripta(ruim, k)
    mostra "passou?!"
quebrou err
    mostra erro_msg(err)
acabou_finalmente`, "deu ruim: nao deu pra decriptar: chave errada ou texto adulterado\n"},
		// token aleatorio: base64url sem padding
		{`mostra tamanho(token_aleatorio())
mostra tamanho(token_aleatorio(16))
mostra token_aleatorio() == token_aleatorio()
mostra busca_regex("^[A-Za-z0-9_-]+$", token_aleatorio(64))`, "43\n22\ndeu_ruim\ndeu_bom\n"},
		// JWT: igualzinho o exemplo do jwt.io, e de volta
		{`bota tok = jwt_assina({"sub": "1234567890", "name": "John Doe", "iat": 1516239022}, "your-256-bit-secret")
mostra tok
bota claims = jwt_confere(tok, "your-256-bit-secret")
mostra claims.name
mostra claims`, "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIiwiaWF0IjoxNTE2MjM5MDIyfQ.SflKxwRJSMeKKF2QT4fwpMeJf36POk6yJV_adQssw5c\n" +
			"John Doe\n" + `{"sub": "1234567890", "name": "John Doe", "iat": 1516239022}` + "\n"},
		// JWT: recusas viram erro "jwt" pegavel
		{`bota tok = jwt_assina({"id": 7}, "certo")
gambiarra tenta(t, s)
    arruma
        funciona jwt_confere(t, s)
    quebrou err
        funciona erro_tipo(err) + ": " + erro_msg(err)
    acabou_finalmente
acabou_finalmente
mostra tenta(tok, "certo")
mostra tenta(tok, "errado")
mostra tenta(jwt_assina({"id": 7}, "certo", {"expira_em": -1}), "certo")
mostra tenta("eyJhbGciOiJub25lIn0.eyJpZCI6N30.", "certo")
mostra tenta("lixo", "certo")
bota c = jwt_confere(jwt_assina({"id": 7}, "certo", {"expira_em": 60}), "certo")
mostra c.exp - c.iat`, `{"id": 7}` + "\n" +
			"jwt: deu ruim: jwt: assinatura invalida (segredo errado ou token adulterado)\n" +
			"jwt: deu ruim: jwt: token expirado\n" +
			"jwt: deu ruim: jwt: alg none nao rola, so HS256\n" +
			"jwt: deu ruim: jwt: token mal formado (quer 3 partes separadas por ponto, veio 1)\n" +
			"60\n"},
		// env com padrao
		{`mostra env("GS_ESSA_NAO_EXISTE_NEM_A_PAU")
mostra env("GS_ESSA_NAO_EXISTE_NEM_A_PAU", 8080)`, "nada\n8080\n"},
	}
	for _, c := range casos {
		esperaNosDois(t, c.src, c.saida, "")
	}
	// erro de uso nao pego: os 2 engines morrem com a mesma mensagem
	esperaNosDois(t, `mostra "antes"
hash_senha(123)`, "antes\n", "hash_senha: senha tem que ser texto, veio NUMERO")
}

// rodaNosDoisCom roda nos 2 engines com argumentos e stderr capturado;
// devolve [tree, vm] de (stdout, stderr, erro, codigo do sai ou -1).
type rodada struct {
	saida, stderr, erro string
	sai                 int
}

func rodaNosDoisCom(t *testing.T, src string, args ...string) [2]rodada {
	t.Helper()
	p := parser.New(lexer.New(src))
	prog := p.ParseProgram()
	if errs := p.Errors(); len(errs) > 0 {
		t.Fatalf("parse: %v", errs)
	}
	var res [2]rodada

	var out, errOut bytes.Buffer
	it := interpreter.New(&out)
	it.DefinirStderr(&errOut)
	it.DefinirArgumentos(args)
	r := it.Eval(prog, object.NewEnvironment())
	res[0] = rodada{sai: -1}
	if s, ok := r.(*object.Sair); ok {
		res[0].sai = s.Codigo
	} else if e, ok := r.(*object.Erro); ok && !e.Handled {
		res[0].erro = e.Message
	}
	res[0].saida, res[0].stderr = out.String(), errOut.String()

	comp := compiler.New()
	if err := comp.Compile(prog); err != nil {
		t.Fatalf("compile: %v", err)
	}
	var out2, errOut2 bytes.Buffer
	iv := interpreter.New(&out2)
	iv.DefinirStderr(&errOut2)
	iv.DefinirArgumentos(args)
	maq := NovaComInterp(comp.Bytecode(), &out2, iv)
	res[1] = rodada{sai: -1}
	if err := maq.Run(); err != nil {
		if sr, ok := err.(SaiRequisicao); ok {
			res[1].sai = sr.Codigo
		} else {
			res[1].erro = err.Error()
		}
	}
	res[1].saida, res[1].stderr = out2.String(), errOut2.String()
	return res
}

var reHoraLog = regexp.MustCompile(`\d{4}-\d\d-\d\dT\d\d:\d\d:\d\d(\.\d{3})?(Z|[+-]\d\d:\d\d)`)

func TestLogNosDois(t *testing.T) {
	src := `log_debug("escondido")
log_info("subiu", {"porta": 8080, "modo": "dev local"})
log_aviso("disco quase cheio", {"livre_pct": 4.5})
log_erro("caiu")
gambiarra trampo(n)
    log_info("trampo", {"n": n})
acabou_finalmente
bota fs = []
pra_cada n em 1..20
    adiciona(fs, bora trampo(n))
acabou_finalmente
espera(fs)
mostra "fim"`
	casos := []struct{ formato, nivel, esp string }{
		{"", "", "HORA INFO subiu porta=8080 modo=\"dev local\"\nHORA AVISO disco quase cheio livre_pct=4.5\nHORA ERRO caiu\n"},
		{"", "erro", "HORA ERRO caiu\n"},
		{"json", "debug", `{"time":"HORA","nivel":"debug","msg":"escondido","campos":{}}` + "\n" +
			`{"time":"HORA","nivel":"info","msg":"subiu","campos":{"porta":8080,"modo":"dev local"}}` + "\n" +
			`{"time":"HORA","nivel":"aviso","msg":"disco quase cheio","campos":{"livre_pct":4.5}}` + "\n" +
			`{"time":"HORA","nivel":"erro","msg":"caiu","campos":{}}` + "\n"},
	}
	for _, c := range casos {
		t.Setenv("GS_LOG_FORMATO", c.formato)
		t.Setenv("GS_LOG_NIVEL", c.nivel)
		for idx, r := range rodaNosDoisCom(t, src) {
			motor := []string{"tree", "vm"}[idx]
			if r.erro != "" || r.saida != "fim\n" {
				t.Fatalf("[%s] erro=%q saida=%q", motor, r.erro, r.saida)
			}
			linhas := strings.Split(strings.TrimSuffix(reHoraLog.ReplaceAllString(r.stderr, "HORA"), "\n"), "\n")
			// as 20 do bora chegam em qualquer ordem, mas cada uma inteira
			var fixas []string
			trampos := 0
			for _, l := range linhas {
				if strings.Contains(l, "trampo") {
					if !regexp.MustCompile(`^(HORA INFO trampo n=\d+|\{"time":"HORA","nivel":"info","msg":"trampo","campos":\{"n":\d+\}\})$`).MatchString(l) {
						t.Fatalf("[%s] linha de log embolada: %q", motor, l)
					}
					trampos++
					continue
				}
				fixas = append(fixas, l)
			}
			esperaTrampos := 20
			if c.nivel == "erro" {
				esperaTrampos = 0
			}
			if trampos != esperaTrampos {
				t.Errorf("[%s] %d linhas de trampo, queria %d", motor, trampos, esperaTrampos)
			}
			if got := strings.Join(fixas, "\n") + "\n"; got != c.esp {
				t.Errorf("[%s] formato=%q nivel=%q\n veio %q\n quer %q", motor, c.formato, c.nivel, got, c.esp)
			}
		}
	}
}

func TestOpcoesNosDois(t *testing.T) {
	src := `bota cfg = opcoes({"porta": 8080, "verboso": deu_ruim, "nome": "api"}, {"porta": "porta do HTTP"})
mostra cfg
mostra cfg.porta + 1`
	casos := []struct {
		args        []string
		saida, erro string
		sai         int
	}{
		{nil, `{"porta": 8080, "verboso": deu_ruim, "nome": "api", "_": []}` + "\n8081\n", "", -1},
		{[]string{"--porta", "9090", "--verboso", "arq.txt", "--nome=tropa"}, `{"porta": 9090, "verboso": deu_bom, "nome": "tropa", "_": [arq.txt]}` + "\n9091\n", "", -1},
		{[]string{"--porta=abc"}, "", "--porta espera numero, veio abc", -1},
		{[]string{"--portaa", "1"}, "", "flag desconhecida --portaa (as que rolam: --porta, --verboso, --nome, --ajuda)", -1},
		{[]string{"--ajuda"}, "uso: [opcoes] [--] [argumentos...]\n\nopcoes:\n" +
			"  --porta <numero>         porta do HTTP (padrao: 8080)\n" +
			"  --verboso, --no-verboso  (padrao: deu_ruim)\n" +
			"  --nome <texto>           (padrao: \"api\")\n" +
			"  --ajuda, -h              mostra essa ajuda e sai\n", "", 0},
	}
	for _, c := range casos {
		for idx, r := range rodaNosDoisCom(t, src, c.args...) {
			motor := []string{"tree", "vm"}[idx]
			if r.saida != c.saida || r.sai != c.sai {
				t.Errorf("[%s] %v: saida %q (sai %d), queria %q (sai %d)", motor, c.args, r.saida, r.sai, c.saida, c.sai)
			}
			if (c.erro == "") != (r.erro == "") || !strings.Contains(r.erro, c.erro) {
				t.Errorf("[%s] %v: erro %q, queria %q", motor, c.args, r.erro, c.erro)
			}
		}
	}
}

// sai() no meio de expressao (`bota x = sai(3)`, argumento de chamada,
// dentro de arruma) desenrola nos 2 engines — o tree-walker guardava o Sair
// na variavel e seguia, o que quebrava `bota cfg = opcoes(...)` com --ajuda.
func TestSaiEmExpressaoNosDois(t *testing.T) {
	casos := []struct {
		src, saida string
		sai        int
	}{
		{"mostra \"a\"\nbota x = sai(3)\nmostra \"nao chega\"", "a\n", 3},
		{"mostra texto(sai(2))\nmostra \"nao chega\"", "", 2},
		{"gambiarra f()\n    bota y = [1, sai(4)]\n    funciona y\nacabou_finalmente\nmostra f()\nmostra \"nao\"", "", 4},
		{"arruma\n    bota z = sai(5)\nquebrou err\n    mostra \"pegou?!\"\nacabou_finalmente\nmostra \"nao\"", "", 5},
		{"bota xs = [3, 1, 2]\nordena_com(xs, gambiarra(a, b) funciona sai(6) acabou_finalmente)\nmostra \"nao\"", "", 6},
	}
	for _, c := range casos {
		for idx, r := range rodaNosDoisCom(t, c.src) {
			motor := []string{"tree", "vm"}[idx]
			if r.saida != c.saida || r.sai != c.sai || r.erro != "" {
				t.Errorf("[%s]\n%s\n veio saida %q sai %d erro %q; queria %q sai %d", motor, c.src, r.saida, r.sai, r.erro, c.saida, c.sai)
			}
		}
	}
}

func TestCarregaEnvNosDois(t *testing.T) {
	dir := t.TempDir()
	arq := filepath.Join(dir, ".env")
	conteudo := "# config de dev\nGS_PARIDADE_PORTA=9090\nexport GS_PARIDADE_NOME=\"api \\\"boa\\\"\"\nGS_PARIDADE_JA=do_arquivo # comentario\n"
	if err := os.WriteFile(arq, []byte(conteudo), 0o600); err != nil {
		t.Fatal(err)
	}
	limpa := func() {
		os.Unsetenv("GS_PARIDADE_PORTA")
		os.Unsetenv("GS_PARIDADE_NOME")
		os.Setenv("GS_PARIDADE_JA", "do_ambiente")
	}
	t.Cleanup(func() {
		os.Unsetenv("GS_PARIDADE_PORTA")
		os.Unsetenv("GS_PARIDADE_NOME")
		os.Unsetenv("GS_PARIDADE_JA")
	})
	src := fmt.Sprintf(`bota r = carrega_env(%q)
mostra r
mostra numero(env("GS_PARIDADE_PORTA")) + 1
mostra env("GS_PARIDADE_JA")
carrega_env(%q, {"sobrescreve": deu_bom})
mostra env("GS_PARIDADE_JA")
arruma
    carrega_env(%q)
quebrou err
    mostra erro_tipo(err)
acabou_finalmente`, arq, arq, filepath.Join(dir, "nao_tem.env"))
	esp := `{"GS_PARIDADE_PORTA": "9090", "GS_PARIDADE_NOME": "api "boa"", "GS_PARIDADE_JA": "do_ambiente"}
9091
do_ambiente
do_arquivo
io
`
	for _, motor := range []func(*testing.T, string) (string, string, string){rodaTWComp, rodaVMComp} {
		limpa()
		_, saida, errStr := motor(t, src)
		if saida != esp || errStr != "" {
			t.Errorf("carrega_env:\n veio %q (erro %q)\n quer %q", saida, errStr, esp)
		}
	}
}
