package main

// gs migra: migracoes de banco em SQL puro (pacote migracao).
//
//	gs migra [--banco URL] [--pasta migracoes] [sobe | desce [n] | status | novo <nome>]
//
// Sem subcomando = sobe. A url e a mesma do conecta() ("sqlite:app.db",
// "postgres://...", "mysql://..."); sem --banco vale a variavel GS_BANCO.

import (
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"gambiarrascript/interpreter"
	"gambiarrascript/migracao"
)

func cmdMigra(args []string) {
	if err := executaMigra(args, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "gs migra: "+err.Error())
		os.Exit(1)
	}
}

const usoMigra = `uso: gs migra [--banco URL] [--pasta migracoes] [sobe | desce [n] | status | novo <nome>]
  sobe          aplica as migracoes pendentes (padrao)
  desce [n]     desfaz as n ultimas aplicadas (padrao 1), com o .desce.sql
  status        lista aplicadas e pendentes
  novo <nome>   cria o proximo par NNN_nome.sobe.sql / NNN_nome.desce.sql
  --banco URL   mesma url do conecta() (sqlite:app.db, postgres://..., mysql://...); padrao: $GS_BANCO
  --pasta DIR   onde ficam os .sql (padrao: migracoes)`

// executaMigra e o gs migra sem os.Exit (testavel).
func executaMigra(args []string, saida io.Writer) error {
	banco := os.Getenv("GS_BANCO")
	pasta := migracao.PastaPadrao
	var resto []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "--banco" || a == "--pasta":
			if i+1 >= len(args) {
				return fmt.Errorf("%s precisa de um valor\n%s", a, usoMigra)
			}
			if a == "--banco" {
				banco = args[i+1]
			} else {
				pasta = args[i+1]
			}
			i++
		case strings.HasPrefix(a, "--banco="):
			banco = strings.TrimPrefix(a, "--banco=")
		case strings.HasPrefix(a, "--pasta="):
			pasta = strings.TrimPrefix(a, "--pasta=")
		case a == "-h" || a == "--help" || a == "ajuda":
			fmt.Fprintln(saida, usoMigra)
			return nil
		case strings.HasPrefix(a, "-"):
			return fmt.Errorf("opcao desconhecida %q\n%s", a, usoMigra)
		default:
			resto = append(resto, a)
		}
	}
	acao := "sobe"
	if len(resto) > 0 {
		acao, resto = resto[0], resto[1:]
	}

	if acao == "novo" {
		if len(resto) == 0 {
			return fmt.Errorf("falta o nome: gs migra novo cria_usuarios")
		}
		sobe, desce, err := migracao.CriaArquivos(pasta, strings.Join(resto, " "))
		if err != nil {
			return err
		}
		fmt.Fprintf(saida, "  %s  (criado)\n  %s  (criado)\n", sobe, desce)
		return nil
	}

	n := 1
	switch acao {
	case "sobe", "status":
		if len(resto) > 0 {
			return fmt.Errorf("%s nao recebe argumento (veio %q)", acao, resto[0])
		}
	case "desce":
		if len(resto) > 1 {
			return fmt.Errorf("desce recebe so a quantidade (veio %q)", strings.Join(resto, " "))
		}
		if len(resto) == 1 {
			v, err := strconv.Atoi(resto[0])
			if err != nil || v < 1 {
				return fmt.Errorf("desce quer um numero >= 1, veio %q", resto[0])
			}
			n = v
		}
	default:
		return fmt.Errorf("subcomando desconhecido %q\n%s", acao, usoMigra)
	}

	if banco == "" {
		return fmt.Errorf("qual banco? passa --banco URL ou define GS_BANCO (ex: sqlite:app.db)")
	}
	db, driver, err := interpreter.AbreBanco(banco)
	if err != nil {
		return err
	}
	defer db.Close()
	m := migracao.Novo(db, driver, pasta)

	switch acao {
	case "sobe":
		feitas, err := m.Sobe()
		for _, mg := range feitas {
			fmt.Fprintf(saida, "  aplicada  %s\n", mg.Rotulo())
		}
		if err != nil {
			return err
		}
		if len(feitas) == 0 {
			fmt.Fprintln(saida, "nada pra aplicar: banco em dia")
		} else {
			fmt.Fprintf(saida, "%d %s\n", len(feitas), pluralMigra(len(feitas), "migracao aplicada", "migracoes aplicadas"))
		}
	case "desce":
		feitas, err := m.Desce(n)
		for _, mg := range feitas {
			fmt.Fprintf(saida, "  desfeita  %s\n", mg.Rotulo())
		}
		if err != nil {
			return err
		}
		if len(feitas) == 0 {
			fmt.Fprintln(saida, "nada pra desfazer: nenhuma migracao aplicada")
		} else {
			fmt.Fprintf(saida, "%d %s\n", len(feitas), pluralMigra(len(feitas), "migracao desfeita", "migracoes desfeitas"))
		}
	case "status":
		est, err := m.Status()
		if err != nil {
			return err
		}
		if len(est) == 0 {
			fmt.Fprintf(saida, "nenhuma migracao em %s\n", pasta)
			return nil
		}
		pendentes, problemas := 0, 0
		for _, e := range est {
			rot := fmt.Sprintf("%03d_%s", e.Versao, e.Nome)
			switch {
			case e.Sumiu:
				problemas++
				fmt.Fprintf(saida, "  [!] %-40s aplicada em %s, mas o arquivo sumiu\n", rot, e.AplicadaEm)
			case e.Alterada:
				problemas++
				fmt.Fprintf(saida, "  [!] %-40s aplicada em %s, mas o arquivo MUDOU depois\n", rot, e.AplicadaEm)
			case e.Aplicada:
				fmt.Fprintf(saida, "  [x] %-40s aplicada em %s\n", rot, e.AplicadaEm)
			default:
				pendentes++
				fmt.Fprintf(saida, "  [ ] %-40s pendente\n", rot)
			}
		}
		fmt.Fprintf(saida, "%d aplicada(s), %d pendente(s)\n", len(est)-pendentes, pendentes)
		if problemas > 0 {
			return fmt.Errorf("%d migracao(oes) aplicada(s) nao bate(m) com o arquivo: sobe/desce vao recusar ate resolver", problemas)
		}
	}
	return nil
}

func pluralMigra(n int, um, varios string) string {
	if n == 1 {
		return um
	}
	return varios
}
