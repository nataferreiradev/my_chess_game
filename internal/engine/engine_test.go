package engine

import (
	"context"
	"strings"
	"testing"
	"time"

	chess "github.com/nataferreiradev/my_chess_game"
)

// implementada vira true quando a busca existir. Enquanto for false, os testes
// de contrato se marcam como pendentes em vez de falhar: eles descrevem o que o
// motor tem de fazer, e são a lista de tarefas da implementação.
//
// Quem implementar: troque para true e faça todos passarem.
const implementada = false

func pendente(t *testing.T) {
	t.Helper()
	if !implementada {
		t.Skip("motor ainda não implementado — ver PROMPT_ENGINE.md")
	}
}

// melhorLance monta a partida da FEN e pede o lance do motor.
func melhorLance(t *testing.T, fen string, opts Options) Result {
	t.Helper()

	game, err := chess.NewGameFromFEN(fen)
	if err != nil {
		t.Fatalf("FEN inválida %q: %v", fen, err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	res, err := New(opts).Best(ctx, game)
	if err != nil {
		t.Fatalf("Best: %v", err)
	}
	return res
}

// TestAchaOMateEmUm é o piso: um motor que não vê o mate na cara não serve.
func TestAchaOMateEmUm(t *testing.T) {
	pendente(t)

	casos := []struct {
		nome, fen, quer string
	}{
		{"mate do corredor", "6k1/5ppp/8/8/8/8/8/R3K3 w - - 0 1", "Ra8#"},
		{"mate da dama", "7k/6pp/8/8/8/8/6PP/5Q1K w - - 0 1", "Qf8#"},
	}

	for _, caso := range casos {
		t.Run(caso.nome, func(t *testing.T) {
			res := melhorLance(t, caso.fen, Level(Dificil).Options())
			if strings.TrimSuffix(res.Move, "#") != strings.TrimSuffix(caso.quer, "#") {
				t.Errorf("jogou %q, o mate era %q", res.Move, caso.quer)
			}
			if res.Mate <= 0 {
				t.Errorf("Mate = %d, queria positivo: o motor devia saber que é mate", res.Mate)
			}
		})
	}
}

// TestPegaAPeçaPendurada: com uma dama de graça no tabuleiro, o motor pega.
func TestPegaAPeçaPendurada(t *testing.T) {
	pendente(t)

	// Os peões estão aí só para a captura não virar material insuficiente: sem
	// eles a posição empata sozinha depois de Bxd5 e o teste não prova nada.
	res := melhorLance(t, "4k3/7p/8/3q4/4B3/8/P7/4K3 w - - 0 1", Level(Medio).Options())
	if res.Move != "Bxd5" {
		t.Errorf("jogou %q, a dama estava de graça em d5", res.Move)
	}
}

// TestNãoDeixaAPrópriaPeçaDeGraça: um lance que pendura a dama por nada só é
// jogado por quem não olha a resposta do adversário.
func TestNãoDeixaAPrópriaPeçaDeGraça(t *testing.T) {
	pendente(t)

	// O bispo preto em b7 varre a diagonal longa: d5 e f3 são casas onde a dama
	// branca morre de graça.
	res := melhorLance(t, "4k3/1b6/8/8/8/8/P7/3QK3 w - - 0 1", Level(Medio).Options())
	for _, pendurada := range []string{"Qd5", "Qf3"} {
		if res.Move == pendurada {
			t.Errorf("jogou %q e entregou a dama ao bispo de b7", res.Move)
		}
	}
}

// TestNãoAfogaComVantagem: com dama e rei contra rei sozinho, afogar é jogar
// fora a partida ganha. É o erro clássico de quem avalia só material.
func TestNãoAfogaComVantagem(t *testing.T) {
	pendente(t)

	game, err := chess.NewGameFromFEN("7k/8/8/8/8/8/5Q2/6K1 w - - 0 1")
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	res, err := New(Level(Medio).Options()).Best(ctx, game)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := game.MakeMove(res.Move); err != nil {
		t.Fatalf("lance %q recusado: %v", res.Move, err)
	}
	if game.Outcome() == chess.Stalemate {
		t.Errorf("jogou %q e afogou o rei preto com dama a mais", res.Move)
	}
}

// TestSempreDevolveLanceLegal joga uma partida inteira do motor contra ele
// mesmo. Nenhum lance pode ser recusado pelas regras, em nenhuma posição.
func TestSempreDevolveLanceLegal(t *testing.T) {
	pendente(t)

	game := chess.NewGame()
	bot := New(Options{Depth: 3, MoveTime: 200 * time.Millisecond, Seed: 1})

	for lance := 0; lance < 120 && !game.Outcome().Over(); lance++ {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		res, err := bot.Best(ctx, game)
		cancel()

		if err != nil {
			t.Fatalf("lance %d: %v", lance, err)
		}
		if _, err := game.MakeMove(res.Move); err != nil {
			t.Fatalf("lance %d: as regras recusaram %q na FEN %q: %v",
				lance, res.Move, game.FEN(), err)
		}
	}
}

// TestNãoMexeNaPartidaRecebida: Best recebe a partida da sala e não pode
// alterá-la — a sala continua dona dela.
func TestNãoMexeNaPartidaRecebida(t *testing.T) {
	pendente(t)

	game := chess.NewGame()
	antes := game.FEN()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if _, err := New(Level(Medio).Options()).Best(ctx, game); err != nil {
		t.Fatal(err)
	}
	if depois := game.FEN(); depois != antes {
		t.Errorf("a partida mudou: %q → %q", antes, depois)
	}
}

// TestRespeitaOTempo: o teto por lance é um teto de verdade. A sala não pode
// ficar pendurada esperando o computador pensar.
func TestRespeitaOTempo(t *testing.T) {
	pendente(t)

	start := time.Now()
	melhorLance(t, chess.InitialFEN, Options{MoveTime: 200 * time.Millisecond})

	if passou := time.Since(start); passou > time.Second {
		t.Errorf("pensou %v com teto de 200ms", passou)
	}
}

// TestParaNoContextoCancelado: a sessão caiu, o contexto morreu, o motor
// devolve o que já tem em vez de continuar queimando CPU.
func TestParaNoContextoCancelado(t *testing.T) {
	pendente(t)

	game := chess.NewGame()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	res, err := New(Options{Depth: 20}).Best(ctx, game)
	if err != nil {
		t.Fatalf("contexto cancelado não é erro, é parar mais cedo: %v", err)
	}
	if res.Move == "" {
		t.Error("devolveu resultado sem lance: mesmo cancelado tem de sair um lance legal")
	}
}

// TestPartidaAcabadaNãoTemLance é o único erro que Best devolve.
func TestPartidaAcabadaNãoTemLance(t *testing.T) {
	pendente(t)

	// Mate do pastor já consumado: as pretas estão matadas.
	game, err := chess.NewGameFromFEN("r1bqkbnr/pppp1Qpp/2n5/4p3/2B1P3/8/PPPP1PPP/RNB1K1NR b KQkq - 0 4")
	if err != nil {
		t.Fatal(err)
	}
	if !game.Outcome().Over() {
		t.Fatalf("a posição de teste devia estar acabada, está %v", game.Outcome())
	}

	if _, err := New(Level(Facil).Options()).Best(context.Background(), game); err == nil {
		t.Error("Best devia recusar uma partida acabada")
	}
}

// TestNívelDifícilGanhaDoFácil é a prova de que o nível quer dizer alguma
// coisa. É lento: roda partidas inteiras.
func TestNívelDifícilGanhaDoFácil(t *testing.T) {
	pendente(t)
	if testing.Short() {
		t.Skip("partidas inteiras: pesado demais para -short")
	}

	t.Skip("TODO(engine): fase 5 — escrever depois que os níveis existirem")
}
