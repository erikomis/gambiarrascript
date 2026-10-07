package main

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"time"

	"gambiarrascript/compiler"
	"gambiarrascript/interpreter"
	"gambiarrascript/lexer"
	"gambiarrascript/lsp"
	"gambiarrascript/object"
	"gambiarrascript/parser"
	"gambiarrascript/vm"
)

// ---------- gs check ----------

// cmdCheck parseia cada arquivo e reporta erros (linha/coluna) + warnings do
// typechecker do LSP. Exit 1 quando ha erro de parse ou diagnostico de erro
// (ex.: mexer em nome cravado); warnings avisam mas nao reprovam.
func cmdCheck(args []string) {
	if len(args) == 0 {
		fmt.Println("uso: gs check <arquivo.gs>...")
		os.Exit(1)
	}
	temErro := false
	for _, arq := range args {
		fonte, err := os.ReadFile(arq)
		if err != nil {
			fmt.Printf("%s: nao consegui abrir: %v\n", arq, err)
			temErro = true
			continue
		}
		p := parser.New(lexer.New(string(fonte)))
		prog := p.ParseProgram()
		errs := p.ErrosDetalhados()
		if len(errs) > 0 {
			temErro = true
			for _, e := range errs {
				fmt.Printf("%s:%d:%d: erro: %s\n", arq, e.Linha, e.Coluna, e.Msg)
			}
			continue
		}
		diags := lsp.Typecheck(prog)
		for _, d := range diags {
			tipo := "aviso"
			if d.Severity == 1 {
				tipo = "erro"
				temErro = true
			}
			// Diagnostico do LSP e 0-based; humano quer 1-based.
			fmt.Printf("%s:%d:%d: %s: %s\n", arq, d.Range.Start.Line+1, d.Range.Start.Character+1, tipo, d.Message)
		}
		if len(diags) == 0 {
			fmt.Printf("%s: suave, zero perrengue\n", arq)
		}
	}
	if temErro {
		os.Exit(1)
	}
}

// ---------- gs init ----------

// cmdInit cria o esqueleto de um projeto: gambiarra.json + principal.gs.
// Nunca sobrescreve arquivo existente.
func cmdInit(args []string) {
	nome := "meu_projeto"
	if len(args) > 0 {
		nome = args[0]
	} else if wd, err := os.Getwd(); err == nil {
		nome = filepath.Base(wd)
	}

	if _, err := os.Stat("gambiarra.json"); err == nil {
		fmt.Println("gambiarra.json ja existe — nao vou passar por cima")
	} else {
		manifesto := map[string]interface{}{
			"nome":         nome,
			"versao":       "0.1.0",
			"principal":    "principal.gs",
			"dependencias": map[string]string{},
		}
		blob, _ := json.MarshalIndent(manifesto, "", "  ")
		if err := os.WriteFile("gambiarra.json", append(blob, '\n'), 0644); err != nil {
			fmt.Println("nao consegui criar gambiarra.json: " + err.Error())
			os.Exit(1)
		}
		fmt.Println("  gambiarra.json  (criado)")
	}

	if _, err := os.Stat("principal.gs"); err == nil {
		fmt.Println("principal.gs ja existe — deixa quieto")
	} else {
		principal := "# " + nome + " — feito com gambiarra e carinho\n" +
			"mostra \"salve, " + nome + "!\"\n"
		if err := os.WriteFile("principal.gs", []byte(principal), 0644); err != nil {
			fmt.Println("nao consegui criar principal.gs: " + err.Error())
			os.Exit(1)
		}
		fmt.Println("  principal.gs    (criado)")
	}
	fmt.Println("pronto! roda com: gs roda principal.gs")
}

// ---------- gs bench ----------

// cmdBench roda o arquivo N vezes (default 10) e reporta min/mediana/media/max.
// A saida do script vai pro ralo (io.Discard) pra nao poluir a medicao.
func cmdBench(args []string) {
	usarVM := true // mede o engine padrao; --tree mede o tree-walker
	n := 10
	arquivo := ""
	for _, a := range args {
		if a == "--vm" {
			usarVM = true
			continue
		}
		if a == "--tree" {
			usarVM = false
			continue
		}
		if v, err := strconv.Atoi(a); err == nil && arquivo != "" {
			n = v
			continue
		}
		if arquivo == "" {
			arquivo = a
		}
	}
	if arquivo == "" {
		fmt.Println("uso: gs bench [--vm] <arquivo.gs> [n]")
		os.Exit(1)
	}
	fonte, err := os.ReadFile(arquivo)
	if err != nil {
		fmt.Printf("nao consegui abrir %q: %v\n", arquivo, err)
		os.Exit(1)
	}
	p := parser.New(lexer.New(string(fonte)))
	prog := p.ParseProgram()
	if errs := p.Errors(); len(errs) != 0 {
		fmt.Println("eita, perrengue de parse:")
		for _, e := range errs {
			fmt.Println("  - " + e)
		}
		os.Exit(1)
	}

	var comp *compiler.Compiler
	if usarVM {
		comp = compiler.New()
		comp.DirBase = filepath.Dir(arquivo)
		if err := comp.Compile(prog); err != nil {
			fmt.Println("a VM nao compilou: " + err.Error())
			os.Exit(1)
		}
	}

	tempos := make([]time.Duration, 0, n)
	for i := 0; i < n; i++ {
		ini := time.Now()
		if usarVM {
			maq := vm.New(comp.Bytecode(), io.Discard)
			if err := maq.Run(); err != nil {
				fmt.Println(err.Error())
				os.Exit(1)
			}
		} else {
			interp := interpreter.New(io.Discard)
			interp.DefinirDirBase(filepath.Dir(arquivo))
			res := interp.Eval(prog, object.NewEnvironment())
			if res != nil && res.Type() == object.ERRO_OBJ {
				fmt.Println("deu ruim: " + res.Inspect())
				os.Exit(1)
			}
		}
		tempos = append(tempos, time.Since(ini))
	}

	sort.Slice(tempos, func(i, j int) bool { return tempos[i] < tempos[j] })
	var total time.Duration
	for _, t := range tempos {
		total += t
	}
	engine := "tree-walker"
	if usarVM {
		engine = "vm"
	}
	fmt.Printf("bench %s (%s, %d rodadas)\n", filepath.Base(arquivo), engine, n)
	fmt.Printf("  min:     %s\n", tempos[0])
	fmt.Printf("  mediana: %s\n", tempos[len(tempos)/2])
	fmt.Printf("  media:   %s\n", total/time.Duration(n))
	fmt.Printf("  max:     %s\n", tempos[len(tempos)-1])
}

// ---------- gs build ----------

// Formato do payload embedado (lido de tras pra frente):
//
//	[binario gs][fonte .gs][8 bytes LE len(fonte)][magic 8 bytes]
const buildMagic = "GSEMBED1"

// rodarEmbedado checa se ESTE executavel carrega um script embedado (gs
// build). Se sim, roda o script com os args da linha de comando e devolve
// true — o main nem processa subcomandos.
func rodarEmbedado() bool {
	eu, err := os.Executable()
	if err != nil {
		return false
	}
	f, err := os.Open(eu)
	if err != nil {
		return false
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil || info.Size() < int64(len(buildMagic))+8 {
		return false
	}
	rodape := make([]byte, len(buildMagic)+8)
	if _, err := f.ReadAt(rodape, info.Size()-int64(len(rodape))); err != nil {
		return false
	}
	if string(rodape[8:]) != buildMagic {
		return false
	}
	tam := int64(binary.LittleEndian.Uint64(rodape[:8]))
	if tam <= 0 || tam > info.Size() {
		return false
	}
	fonte := make([]byte, tam)
	if _, err := f.ReadAt(fonte, info.Size()-int64(len(rodape))-tam); err != nil {
		return false
	}

	p := parser.New(lexer.New(string(fonte)))
	prog := p.ParseProgram()
	if errs := p.Errors(); len(errs) != 0 {
		fmt.Println("o script embedado ta com perrengue (como?):")
		for _, e := range errs {
			fmt.Println("  - " + e)
		}
		os.Exit(1)
	}
	wd, _ := os.Getwd()
	interp := interpreter.New(os.Stdout)
	interp.DefinirArgumentos(os.Args[1:])
	interp.DefinirDirBase(wd)

	// binario standalone roda na VM, igual `gs roda` — antes caia no
	// tree-walker, o que fazia o executavel "compilado" ser ~10x MAIS LENTO
	// que rodar o .gs solto. Se a compilacao falhar (construcao que so o
	// tree-walker aceita), cai pro interpretador em vez de morrer.
	comp := compiler.New()
	comp.DirBase = wd
	if err := comp.Compile(prog); err == nil {
		maquina := vm.NovaComInterp(comp.Bytecode(), os.Stdout, interp)
		if err := maquina.Run(); err != nil {
			trataSaiVM(err)
			reportaErroVM(err)
			os.Exit(1)
		}
		return true
	}

	res := interp.Eval(prog, object.NewEnvironment())
	if res != nil && res.Type() == object.ERRO_OBJ {
		fmt.Println(res.Inspect())
		os.Exit(1)
	}
	return true
}
