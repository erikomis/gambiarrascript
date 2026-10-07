package vm

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/coder/websocket"
)

// Origem do websocket: o navegador manda o cookie no handshake vindo de
// qualquer site, entao o cors() aberto NAO pode liberar o rota_ws pra outra
// origem (cross-site WebSocket hijacking). So a lista da propria rota_ws
// libera, e "*" so quando pedido com todas as letras.
func TestWebSocketOrigemNaoVazaPeloCorsAberto(t *testing.T) {
	fonte := `cors()
gambiarra eco(ws, pedido)
    envia(ws, "ok")
acabou_finalmente
rota_ws("/padrao", eco)
rota_ws("/lista", eco, {"origens": ["https://app.com"]})
rota_ws("/todas", eco, {"origens": ["*"]})
`
	servidorNosDois(t, fonte, func(t *testing.T, c ctxServidor) {
		ctx, cancela := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancela()
		abre := func(caminho, origem string) bool {
			opcoes := &websocket.DialOptions{HTTPHeader: http.Header{"Origin": {origem}}}
			conn, _, err := websocket.Dial(ctx, wsURL(c.base, caminho), opcoes)
			if err != nil {
				return false
			}
			conn.CloseNow()
			return true
		}
		casos := []struct {
			caminho, origem string
			abre            bool
		}{
			{"/padrao", "https://mal.com", false}, // cors() aberto nao vale pro ws
			{"/lista", "https://app.com", true},
			{"/lista", "https://mal.com", false},
			{"/todas", "https://mal.com", true}, // opt-in explicito
		}
		for _, cs := range casos {
			if got := abre(cs.caminho, cs.origem); got != cs.abre {
				t.Errorf("%s com Origin %s: abriu=%v, esperado %v", cs.caminho, cs.origem, got, cs.abre)
			}
		}
	})
}

// cors() com credenciais e qualquer origem ecoaria a origem de qualquer site
// com Allow-Credentials: pedido logado em nome do usuario. Vira erro.
func TestCorsCredenciaisExigeListaDeOrigens(t *testing.T) {
	esperaNosDois(t, `cors({"credenciais": deu_bom})`, "", `"credenciais" com qualquer origem`)
	esperaNosDois(t, `cors({"origens": ["*"], "credenciais": deu_bom})`, "", `"credenciais" com qualquer origem`)
	esperaNosDois(t, `cors({"origens": ["https://app.com"], "credenciais": deu_bom})`, "", "")
	esperaNosDois(t, `rota_ws("/x", gambiarra(ws, p) acabou_finalmente, {"origem": ["*"]})`, "", "opcao desconhecida")
}
