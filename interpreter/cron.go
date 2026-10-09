package interpreter

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Expressao cron do agenda("...", gambiarra): os 5 campos classicos
//
//	minuto hora dia mes dia_semana
//	0-59   0-23 1-31 1-12 0-6 (0 = domingo; 7 tambem vale domingo)
//
// Cada campo aceita `*`, numero, lista (`1,15,30`), faixa (`9-18`) e passo
// (`*/5`, `0-30/10`, `5/15` = de 5 ate o fim, de 15 em 15). Igual ao cron do
// Unix, se dia E dia_semana forem restritos, basta UM dos dois bater.
// Atalhos: @minuto, @hora (@horario), @diario, @semanal, @mensal, @anual —
// e os nomes em ingles (@hourly, @daily, @weekly, @monthly, @yearly).
// O horario e sempre o do fuso local da maquina (TZ).
type cronExpr struct {
	minuto, hora, dia, mes, semana uint64 // bit n ligado = valor n bate
	diaLivre, semanaLivre          bool   // campo era `*` (sem passo)
}

var atalhosCron = map[string]string{
	"@minuto":  "* * * * *",
	"@hora":    "0 * * * *",
	"@horario": "0 * * * *",
	"@hourly":  "0 * * * *",
	"@diario":  "0 0 * * *",
	"@daily":   "0 0 * * *",
	"@semanal": "0 0 * * 0",
	"@weekly":  "0 0 * * 0",
	"@mensal":  "0 0 1 * *",
	"@monthly": "0 0 1 * *",
	"@anual":   "0 0 1 1 *",
	"@yearly":  "0 0 1 1 *",
}

// parseCron le a expressao (5 campos ou um atalho com @).
func parseCron(texto string) (*cronExpr, error) {
	s := strings.TrimSpace(texto)
	if strings.HasPrefix(s, "@") {
		exp, ok := atalhosCron[strings.ToLower(s)]
		if !ok {
			return nil, fmt.Errorf("atalho %q nao existe (tem @minuto, @hora, @diario, @semanal, @mensal, @anual)", s)
		}
		s = exp
	}
	campos := strings.Fields(s)
	if len(campos) != 5 {
		return nil, fmt.Errorf("cron quer 5 campos (minuto hora dia mes dia_semana), veio %d em %q", len(campos), texto)
	}
	c := &cronExpr{}
	var err error
	if c.minuto, _, err = campoCron(campos[0], "minuto", 0, 59); err != nil {
		return nil, err
	}
	if c.hora, _, err = campoCron(campos[1], "hora", 0, 23); err != nil {
		return nil, err
	}
	if c.dia, c.diaLivre, err = campoCron(campos[2], "dia", 1, 31); err != nil {
		return nil, err
	}
	if c.mes, _, err = campoCron(campos[3], "mes", 1, 12); err != nil {
		return nil, err
	}
	if c.semana, c.semanaLivre, err = campoCron(campos[4], "dia_semana", 0, 7); err != nil {
		return nil, err
	}
	if c.semana&(1<<7) != 0 { // 7 = domingo tambem
		c.semana |= 1
	}
	return c, nil
}

// campoCron devolve o bitset do campo e se ele era um `*` puro.
func campoCron(campo, nome string, min, max int) (uint64, bool, error) {
	var bits uint64
	for _, parte := range strings.Split(campo, ",") {
		faixa, passoTxt, temPasso := strings.Cut(parte, "/")
		passo := 1
		if temPasso {
			p, err := strconv.Atoi(passoTxt)
			if err != nil || p <= 0 {
				return 0, false, fmt.Errorf("cron: passo %q invalido no campo %s", passoTxt, nome)
			}
			passo = p
		}
		ini, fim := min, max
		switch {
		case faixa == "*":
		case strings.Contains(faixa, "-"):
			a, b, _ := strings.Cut(faixa, "-")
			var err error
			if ini, err = numeroCron(a, nome, min, max); err != nil {
				return 0, false, err
			}
			if fim, err = numeroCron(b, nome, min, max); err != nil {
				return 0, false, err
			}
			if ini > fim {
				return 0, false, fmt.Errorf("cron: faixa %q ao contrario no campo %s", faixa, nome)
			}
		default:
			n, err := numeroCron(faixa, nome, min, max)
			if err != nil {
				return 0, false, err
			}
			ini = n
			if !temPasso { // `5/15` vai ate o fim; `5` e so o 5
				fim = n
			}
		}
		for v := ini; v <= fim; v += passo {
			bits |= 1 << uint(v)
		}
	}
	return bits, campo == "*", nil
}

func numeroCron(txt, nome string, min, max int) (int, error) {
	n, err := strconv.Atoi(txt)
	if err != nil {
		return 0, fmt.Errorf("cron: %q nao e numero no campo %s", txt, nome)
	}
	if n < min || n > max {
		return 0, fmt.Errorf("cron: %d fora do campo %s (%d-%d)", n, nome, min, max)
	}
	return n, nil
}

func (c *cronExpr) bateDia(t time.Time) bool {
	dia := c.dia&(1<<uint(t.Day())) != 0
	semana := c.semana&(1<<uint(t.Weekday())) != 0
	switch {
	case c.diaLivre && c.semanaLivre:
		return true
	case c.diaLivre:
		return semana
	case c.semanaLivre:
		return dia
	}
	return dia || semana // os dois restritos: basta um (regra do cron)
}

// proximo devolve o primeiro minuto cheio DEPOIS de t (no fuso de t) que
// bate com a expressao. Zero se nao achar nada em 5 anos (ex.: 30 de
// fevereiro).
func (c *cronExpr) proximo(t time.Time) time.Time {
	loc := t.Location()
	// proximo minuto cheio, estritamente depois de t
	t = t.Add(time.Minute - time.Duration(t.Second())*time.Second - time.Duration(t.Nanosecond()))
	limite := t.Year() + 5
	for t.Year() <= limite {
		if c.mes&(1<<uint(t.Month())) == 0 {
			t = time.Date(t.Year(), t.Month()+1, 1, 0, 0, 0, 0, loc)
			continue
		}
		if !c.bateDia(t) {
			t = time.Date(t.Year(), t.Month(), t.Day()+1, 0, 0, 0, 0, loc)
			continue
		}
		if c.hora&(1<<uint(t.Hour())) == 0 {
			// soma (e nao time.Date) pra andar pra frente mesmo no horario de verao
			t = t.Add(time.Duration(60-t.Minute()) * time.Minute)
			continue
		}
		if c.minuto&(1<<uint(t.Minute())) == 0 {
			t = t.Add(time.Minute)
			continue
		}
		return t
	}
	return time.Time{}
}
