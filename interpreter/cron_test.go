package interpreter

import (
	"testing"
	"time"
)

func TestCronProximo(t *testing.T) {
	// quinta-feira, 8 de outubro de 2026, 10:17:42 (relogio fixo, UTC)
	base := time.Date(2026, 10, 8, 10, 17, 42, 0, time.UTC)
	casos := []struct {
		expr, quer string
	}{
		{"* * * * *", "2026-10-08 10:18"},
		{"*/5 * * * *", "2026-10-08 10:20"},
		{"0 * * * *", "2026-10-08 11:00"},
		{"@hora", "2026-10-08 11:00"},
		{"30 9 * * *", "2026-10-09 09:30"},
		{"@diario", "2026-10-09 00:00"},
		{"0 9-18/3 * * *", "2026-10-08 12:00"},
		{"15,45 10 * * *", "2026-10-08 10:45"},
		{"0 8 * * 1-5", "2026-10-09 08:00"},   // sexta
		{"0 8 * * 6,0", "2026-10-10 08:00"},   // sabado
		{"0 8 * * 7", "2026-10-11 08:00"},     // 7 = domingo
		{"0 0 1 * *", "2026-11-01 00:00"},     // @mensal
		{"0 0 1 1 *", "2027-01-01 00:00"},     // @anual
		{"0 0 29 2 *", "2028-02-29 00:00"},    // bissexto
		{"0 0 13 * 5", "2026-10-09 00:00"},    // dia 13 OU sexta: sexta 9 vem antes
		{"5/20 10 * * *", "2026-10-08 10:25"}, // 5, 25, 45
		{"@semanal", "2026-10-11 00:00"},
	}
	for _, c := range casos {
		e, err := parseCron(c.expr)
		if err != nil {
			t.Fatalf("%q: %v", c.expr, err)
		}
		got := e.proximo(base).Format("2006-01-02 15:04")
		if got != c.quer {
			t.Errorf("%q: quer %s, veio %s", c.expr, c.quer, got)
		}
	}
}

func TestCronProximoEstritamenteDepois(t *testing.T) {
	e, _ := parseCron("0 10 * * *")
	base := time.Date(2026, 10, 8, 10, 0, 0, 0, time.UTC) // exatamente no horario
	if got := e.proximo(base); !got.Equal(time.Date(2026, 10, 9, 10, 0, 0, 0, time.UTC)) {
		t.Fatalf("veio %v", got)
	}
}

func TestCronFusoLocal(t *testing.T) {
	sp := time.FixedZone("BRT", -3*3600)
	e, _ := parseCron("0 9 * * *")
	base := time.Date(2026, 10, 8, 10, 0, 0, 0, sp)
	got := e.proximo(base)
	if got.Format("2006-01-02 15:04 -0700") != "2026-10-09 09:00 -0300" {
		t.Fatalf("veio %v", got)
	}
}

func TestCronNuncaBate(t *testing.T) {
	e, err := parseCron("0 0 30 2 *")
	if err != nil {
		t.Fatal(err)
	}
	if got := e.proximo(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)); !got.IsZero() {
		t.Fatalf("30 de fevereiro nao existe, veio %v", got)
	}
}

func TestCronInvalido(t *testing.T) {
	for _, s := range []string{"", "* * * *", "60 * * * *", "* 24 * * *", "* * 0 * *",
		"* * * 13 *", "* * * * 8", "*/0 * * * *", "a * * * *", "5-2 * * * *", "@nunca"} {
		if _, err := parseCron(s); err == nil {
			t.Errorf("%q deveria dar erro", s)
		}
	}
}
