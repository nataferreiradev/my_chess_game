package my_chess_game

import (
	"strings"
	"testing"

	parser "github.com/nataferreiradev/chess_notation_parser"
)

// --- ajudantes ---------------------------------------------------------------

func mustGame(t *testing.T, fen string) *RunningGame {
	t.Helper()
	game, err := NewGameFromFEN(fen)
	if err != nil {
		t.Fatalf("NewGameFromFEN(%q) erro inesperado: %v", fen, err)
	}
	return game
}

// mustPlay joga a sequência e falha no primeiro lance recusado.
func mustPlay(t *testing.T, game *RunningGame, moves ...string) AppliedMove {
	t.Helper()
	var last AppliedMove
	for _, move := range moves {
		applied, err := game.MakeMove(move)
		if err != nil {
			t.Fatalf("MakeMove(%q) erro inesperado: %v", move, err)
		}
		last = applied
	}
	return last
}

// refuse exige que o lance seja recusado e que a partida não mude.
func refuse(t *testing.T, game *RunningGame, move string) error {
	t.Helper()
	before := game.FEN()
	applied, err := game.MakeMove(move)
	if err == nil {
		t.Fatalf("MakeMove(%q) deveria ser recusado, devolveu %+v", move, applied)
	}
	if after := game.FEN(); after != before {
		t.Errorf("o lance recusado %q mudou a partida:\n antes: %s\ndepois: %s", move, before, after)
	}
	return err
}

func assertPiece(t *testing.T, game *RunningGame, square string, want parser.Piece, color Color) {
	t.Helper()
	idx, err := ParseSquareName(square)
	if err != nil {
		t.Fatalf("casa inválida no teste: %q", square)
	}
	piece, ok := game.PieceAt(idx)
	if !ok {
		t.Fatalf("%s está vazia, esperado %s das %v", square, pieceName(want), color)
	}
	if piece.Piece != want || piece.Color != color {
		t.Errorf("%s = %s das %v, esperado %s das %v", square, pieceName(piece.Piece), piece.Color, pieceName(want), color)
	}
}

func assertEmpty(t *testing.T, game *RunningGame, squares ...string) {
	t.Helper()
	for _, square := range squares {
		idx, err := ParseSquareName(square)
		if err != nil {
			t.Fatalf("casa inválida no teste: %q", square)
		}
		if piece, ok := game.PieceAt(idx); ok {
			t.Errorf("%s deveria estar vazia, tem %s das %v", square, pieceName(piece.Piece), piece.Color)
		}
	}
}

// --- movimento básico --------------------------------------------------------

func TestPeaoAndaEEscreveNoTabuleiro(t *testing.T) {
	game := NewGame()

	applied := mustPlay(t, game, "e4")

	assertEmpty(t, game, "e2")
	assertPiece(t, game, "e4", parser.Pawn, White)

	if applied.From != 52 || applied.To != 36 {
		t.Errorf("e4 foi de %d para %d, esperado 52 para 36", applied.From, applied.To)
	}
	if applied.Notation != "e4" {
		t.Errorf("Notation = %q, esperado \"e4\"", applied.Notation)
	}
	if applied.Player != White {
		t.Errorf("Player = %v, esperado brancas", applied.Player)
	}
}

func TestAvancoDuploSoDaFileiraInicial(t *testing.T) {
	game := NewGame()
	mustPlay(t, game, "e4", "e5", "d4", "d5")

	// O peão de e4 já saiu da fileira inicial: não pode dar dois passos.
	refuse(t, game, "e6")
}

func TestAvancoDuploBloqueado(t *testing.T) {
	// Peça preta em e3 bloqueia o avanço duplo do peão de e2.
	game := mustGame(t, "rnbqkbnr/pppppppp/8/8/8/4n3/PPPPPPPP/RNBQKB1R w KQkq - 0 1")
	refuse(t, game, "e4")
	refuse(t, game, "e3")
}

func TestNaoDaParaMoverPecaDoAdversario(t *testing.T) {
	game := NewGame()

	// É a vez das brancas; e5 é um lance de peão preto.
	err := refuse(t, game, "e5")
	if !strings.Contains(err.Error(), "brancas") {
		t.Errorf("a mensagem deveria falar de quem é a vez, foi: %v", err)
	}

	if game.CurrentPlayer() != White {
		t.Error("um lance recusado não pode passar a vez")
	}
}

func TestBloqueioDeCaminho(t *testing.T) {
	game := NewGame()

	// A torre de a1 está trancada pelos próprios peões.
	refuse(t, game, "Ra3")
	// O bispo de c1 também.
	refuse(t, game, "Bb2")
	// O cavalo pula por cima.
	mustPlay(t, game, "Nf3")
	assertPiece(t, game, "f3", parser.Knight, White)
}

func TestCaptura(t *testing.T) {
	game := NewGame()
	mustPlay(t, game, "e4", "d5")

	applied := mustPlay(t, game, "exd5")

	assertPiece(t, game, "d5", parser.Pawn, White)
	assertEmpty(t, game, "e4")

	if applied.Captured == nil {
		t.Fatal("exd5 deveria registrar a peça capturada")
	}
	if applied.Captured.Piece != parser.Pawn || applied.Captured.Color != Black {
		t.Errorf("Captured = %+v, esperado peão preto", *applied.Captured)
	}
	if applied.CapturedAt != applied.To {
		t.Errorf("CapturedAt = %d, numa captura normal deveria ser igual a To = %d", applied.CapturedAt, applied.To)
	}
}

func TestNotacaoTemDeCombinarComOTabuleiro(t *testing.T) {
	game := NewGame()
	mustPlay(t, game, "e4", "d5")

	// Escrever "x" onde não há captura.
	if err := refuse(t, game, "exf5"); !strings.Contains(err.Error(), "alcança") {
		t.Errorf("exf5 não existe, foi: %v", err)
	}

	// E o contrário: omitir o "x" de uma captura de verdade.
	if err := refuse(t, game, "ed5"); !strings.Contains(err.Error(), "captura") {
		t.Errorf("a mensagem deveria explicar que falta o x, foi: %v", err)
	}
}

// --- desambiguação -----------------------------------------------------------

func TestDesambiguacaoPorColuna(t *testing.T) {
	// Cavalos em b1 e f3, os dois alcançam d2.
	game := mustGame(t, "4k3/8/8/8/8/5N2/8/1N2K3 w - - 0 1")

	if err := refuse(t, game, "Nd2"); !strings.Contains(err.Error(), "ambíguo") {
		t.Errorf("Nd2 deveria ser recusado como ambíguo, foi: %v", err)
	}

	applied := mustPlay(t, game, "Nbd2")
	assertPiece(t, game, "d2", parser.Knight, White)
	assertEmpty(t, game, "b1")
	if applied.Notation != "Nbd2" {
		t.Errorf("Notation = %q, esperado \"Nbd2\"", applied.Notation)
	}
}

func TestDesambiguacaoPorLinha(t *testing.T) {
	// Torres em a1 e a5, as duas alcançam a3. Só a linha distingue.
	game := mustGame(t, "4k3/8/8/R7/8/8/8/R3K3 w - - 0 1")

	if err := refuse(t, game, "Ra3"); !strings.Contains(err.Error(), "ambíguo") {
		t.Errorf("Ra3 deveria ser ambíguo, foi: %v", err)
	}

	applied := mustPlay(t, game, "R1a3")
	assertPiece(t, game, "a3", parser.Rook, White)
	assertPiece(t, game, "a5", parser.Rook, White)
	if applied.Notation != "R1a3" {
		t.Errorf("Notation = %q, esperado \"R1a3\"", applied.Notation)
	}
}

// --- roque -------------------------------------------------------------------

func TestRoquePequenoBranco(t *testing.T) {
	game := mustGame(t, "r3k2r/8/8/8/8/8/8/R3K2R w KQkq - 0 1")

	applied := mustPlay(t, game, "O-O")

	assertPiece(t, game, "g1", parser.King, White)
	assertPiece(t, game, "f1", parser.Rook, White)
	assertEmpty(t, game, "e1", "h1")

	if applied.Castle != parser.CastleKingside {
		t.Errorf("Castle = %v, esperado O-O", applied.Castle)
	}
	if applied.RookFrom != 63 || applied.RookTo != 61 {
		t.Errorf("torre foi de %d para %d, esperado 63 para 61", applied.RookFrom, applied.RookTo)
	}
	if applied.Notation != "O-O" {
		t.Errorf("Notation = %q, esperado \"O-O\"", applied.Notation)
	}
}

func TestRoqueGrandeNasDuasCores(t *testing.T) {
	game := mustGame(t, "r3k2r/8/8/8/8/8/8/R3K2R w KQkq - 0 1")

	mustPlay(t, game, "O-O-O")
	assertPiece(t, game, "c1", parser.King, White)
	assertPiece(t, game, "d1", parser.Rook, White)
	assertEmpty(t, game, "e1", "a1", "b1")

	// As pretas roqueiam numa partida à parte: depois do roque grande branco a
	// torre de d1 passa a atacar d8, casa que o rei preto atravessaria.
	pretas := mustGame(t, "r3k2r/8/8/8/8/8/8/R3K2R b KQkq - 0 1")
	mustPlay(t, pretas, "O-O-O")
	assertPiece(t, pretas, "c8", parser.King, Black)
	assertPiece(t, pretas, "d8", parser.Rook, Black)
	assertEmpty(t, pretas, "e8", "a8", "b8")
}

// O roque grande branco tranca o roque grande preto: a torre que chega em d1
// ataca a casa d8, por onde o rei preto teria de passar.
func TestRoqueGrandeBrancoImpedeORoqueGrandePreto(t *testing.T) {
	game := mustGame(t, "r3k2r/8/8/8/8/8/8/R3K2R w KQkq - 0 1")

	mustPlay(t, game, "O-O-O")
	refuse(t, game, "O-O-O")
	mustPlay(t, game, "O-O")
}

func TestRoquePerdeODireitoQuandoARoqueSeMove(t *testing.T) {
	game := mustGame(t, "r3k2r/8/8/8/8/8/8/R3K2R w KQkq - 0 1")

	mustPlay(t, game, "Rh2", "Rh7", "Rh1", "Rh8")

	// A torre do lado do rei voltou para casa, mas o direito já se perdeu.
	refuse(t, game, "O-O")
	// O roque grande continua de pé.
	mustPlay(t, game, "O-O-O")
}

func TestRoquePerdeODireitoQuandoOReiSeMove(t *testing.T) {
	game := mustGame(t, "r3k2r/8/8/8/8/8/8/R3K2R w KQkq - 0 1")

	mustPlay(t, game, "Kd1", "Ke7", "Ke1", "Ke8")

	refuse(t, game, "O-O")
	refuse(t, game, "O-O-O")
}

func TestRoqueComCaminhoOcupado(t *testing.T) {
	game := mustGame(t, "r3k2r/8/8/8/8/8/8/R3KB1R w KQkq - 0 1")
	refuse(t, game, "O-O")
	mustPlay(t, game, "O-O-O")
}

func TestRoqueComOReiEmXeque(t *testing.T) {
	// Torre preta em e8 dá xeque na coluna do rei branco.
	game := mustGame(t, "4r3/8/8/8/8/8/8/R3K2R w KQ - 0 1")
	refuse(t, game, "O-O")
	refuse(t, game, "O-O-O")
}

func TestRoqueAtravessandoCasaAtacada(t *testing.T) {
	// Torre preta em f8 ataca f1, casa que o rei atravessaria no roque pequeno.
	game := mustGame(t, "5r2/8/8/8/8/8/8/R3K2R w KQ - 0 1")
	refuse(t, game, "O-O")
	mustPlay(t, game, "O-O-O")
}

func TestRoqueGrandeComB1Atacada(t *testing.T) {
	// b1 atacada não impede o roque grande: o rei não passa por b1.
	game := mustGame(t, "1r6/8/8/8/8/8/8/R3K2R w KQ - 0 1")
	mustPlay(t, game, "O-O-O")
	assertPiece(t, game, "c1", parser.King, White)
}

func TestRoquePerdeODireitoQuandoATorreEhCapturada(t *testing.T) {
	// Bispo branco em g7 captura a torre de h8.
	game := mustGame(t, "r3k2r/6B1/8/8/8/8/8/4K3 w kq - 0 1")

	mustPlay(t, game, "Bxh8")
	refuse(t, game, "O-O")
	mustPlay(t, game, "O-O-O")
}

// --- en passant --------------------------------------------------------------

func TestEnPassant(t *testing.T) {
	game := mustGame(t, "4k3/3p4/8/4P3/8/8/8/4K3 b - - 0 1")

	mustPlay(t, game, "d5")
	if got := game.FEN(); !strings.Contains(got, " d6 ") {
		t.Errorf("o avanço duplo deveria abrir a casa d6 para en passant, FEN = %q", got)
	}

	applied := mustPlay(t, game, "exd6")

	assertPiece(t, game, "d6", parser.Pawn, White)
	assertEmpty(t, game, "e5", "d5")

	if !applied.EnPassant {
		t.Error("EnPassant deveria ser true")
	}
	if applied.CapturedAt == applied.To {
		t.Error("no en passant a peça capturada não está na casa de destino")
	}
	if want, _ := ParseSquareName("d5"); applied.CapturedAt != want {
		t.Errorf("CapturedAt = %s, esperado d5", SquareName(applied.CapturedAt))
	}
	if applied.Captured == nil || applied.Captured.Color != Black {
		t.Errorf("Captured = %+v, esperado peão preto", applied.Captured)
	}
}

func TestEnPassantSoValeNoLanceSeguinte(t *testing.T) {
	game := mustGame(t, "4k3/3p4/8/4P3/8/8/8/4K3 b - - 0 1")

	mustPlay(t, game, "d5", "Ke2", "Ke7")

	// A janela passou.
	refuse(t, game, "exd6")
}

// --- promoção ----------------------------------------------------------------

func TestPromocao(t *testing.T) {
	game := mustGame(t, "4k3/P7/8/8/8/8/8/4K3 w - - 0 1")

	applied := mustPlay(t, game, "a8=Q")

	assertPiece(t, game, "a8", parser.Queen, White)
	if applied.Promotion != parser.Queen {
		t.Errorf("Promotion = %q, esperado dama", string(byte(applied.Promotion)))
	}
	if applied.Piece != parser.Pawn {
		t.Errorf("Piece = %q, a peça movida é o peão", string(byte(applied.Piece)))
	}
}

func TestPromocaoPrecisaDizerAPeca(t *testing.T) {
	game := mustGame(t, "4k3/P7/8/8/8/8/8/4K3 w - - 0 1")

	err := refuse(t, game, "a8")
	if !strings.Contains(err.Error(), "=Q") {
		t.Errorf("a mensagem deveria sugerir a notação da promoção, foi: %v", err)
	}
}

func TestPromocaoNasQuatroPecas(t *testing.T) {
	for _, tt := range []struct {
		notation string
		want     parser.Piece
	}{
		{"a8=Q", parser.Queen},
		{"a8=R", parser.Rook},
		{"a8=B", parser.Bishop},
		{"a8=N", parser.Knight},
	} {
		t.Run(tt.notation, func(t *testing.T) {
			game := mustGame(t, "4k3/P7/8/8/8/8/8/4K3 w - - 0 1")
			mustPlay(t, game, tt.notation)
			assertPiece(t, game, "a8", tt.want, White)
		})
	}
}

func TestPromocaoComCaptura(t *testing.T) {
	game := mustGame(t, "1r2k3/P7/8/8/8/8/8/4K3 w - - 0 1")

	applied := mustPlay(t, game, "axb8=N")

	assertPiece(t, game, "b8", parser.Knight, White)
	if applied.Captured == nil || applied.Captured.Piece != parser.Rook {
		t.Errorf("Captured = %+v, esperado torre preta", applied.Captured)
	}
	if applied.Notation != "axb8=N" {
		t.Errorf("Notation = %q, esperado \"axb8=N\"", applied.Notation)
	}
}

func TestPeaoPretoPromoveNaLinha1(t *testing.T) {
	game := mustGame(t, "4k3/8/8/8/8/8/7p/4K3 b - - 0 1")
	mustPlay(t, game, "h1=Q")
	assertPiece(t, game, "h1", parser.Queen, Black)
}

// --- xeque e legalidade ------------------------------------------------------

func TestNaoDaParaDeixarOProprioReiEmXeque(t *testing.T) {
	// O cavalo de e2 está cravado: a torre preta de e8 mira o rei de e1.
	game := mustGame(t, "4r3/8/8/8/8/8/4N3/4K3 w - - 0 1")

	refuse(t, game, "Nc3")
	refuse(t, game, "Nd4")

	// Sair da coluna com o rei é legal.
	mustPlay(t, game, "Kd1")
}

func TestXequeEhDetectado(t *testing.T) {
	game := mustGame(t, "4k3/8/8/8/8/8/8/R3K3 w - - 0 1")

	applied := mustPlay(t, game, "Ra8")

	if !applied.Check {
		t.Error("Ra8 dá xeque no rei de e8")
	}
	if applied.Notation != "Ra8+" {
		t.Errorf("Notation = %q, esperado \"Ra8+\"", applied.Notation)
	}
	if !game.InCheck() {
		t.Error("InCheck() deveria ser true para as pretas")
	}
	if game.Outcome() != Ongoing {
		t.Errorf("Outcome = %v, o rei ainda escapa por b7", game.Outcome())
	}
}

func TestEmXequeSoValemLancesQueTiramODoXeque(t *testing.T) {
	game := mustGame(t, "4k3/8/8/8/8/8/8/R3K3 w - - 0 1")
	mustPlay(t, game, "Ra8")

	// Ficar na linha 8 não tira do xeque; sair dela, sim.
	refuse(t, game, "Kf8")
	refuse(t, game, "Kd8")
	mustPlay(t, game, "Ke7")
}

// --- fim de jogo -------------------------------------------------------------

func TestMateDoPastor(t *testing.T) {
	game := NewGame()

	applied := mustPlay(t, game, "e4", "e5", "Bc4", "Nc6", "Qh5", "Nf6", "Qxf7")

	if applied.Outcome != Checkmate {
		t.Fatalf("Outcome = %v, esperado xeque-mate", applied.Outcome)
	}
	if applied.Notation != "Qxf7#" {
		t.Errorf("Notation = %q, esperado \"Qxf7#\"", applied.Notation)
	}
	if game.Outcome() != Checkmate || !game.Outcome().Over() {
		t.Errorf("a partida deveria estar encerrada, Outcome = %v", game.Outcome())
	}
}

func TestMateDoTolo(t *testing.T) {
	game := NewGame()

	applied := mustPlay(t, game, "f3", "e5", "g4", "Qh4")

	if applied.Outcome != Checkmate {
		t.Errorf("Outcome = %v, esperado xeque-mate", applied.Outcome)
	}
	if applied.Player != Black {
		t.Errorf("quem deu o mate foram as pretas, Player = %v", applied.Player)
	}
}

func TestPartidaEncerradaRecusaNovosLances(t *testing.T) {
	game := NewGame()
	mustPlay(t, game, "f3", "e5", "g4", "Qh4")

	err := refuse(t, game, "Nc3")
	if !strings.Contains(err.Error(), "acabou") {
		t.Errorf("a mensagem deveria dizer que a partida acabou, foi: %v", err)
	}
}

func TestAfogamento(t *testing.T) {
	// Rei preto em h8 sem lance legal e sem estar em xeque depois de Qg6.
	game := mustGame(t, "7k/8/8/8/8/8/5Q2/K7 w - - 0 1")

	applied := mustPlay(t, game, "Qf7")

	if applied.Outcome != Stalemate {
		t.Errorf("Outcome = %v, esperado afogamento", applied.Outcome)
	}
	if applied.Check {
		t.Error("no afogamento o rei não está em xeque")
	}
}

func TestMaterialInsuficiente(t *testing.T) {
	tests := []struct {
		name string
		fen  string
		want bool
	}{
		{"rei contra rei", "4k3/8/8/8/8/8/8/4K3 w - - 0 1", true},
		{"rei e bispo contra rei", "4k3/8/8/8/8/8/8/2B1K3 w - - 0 1", true},
		{"rei e cavalo contra rei", "4k3/8/8/8/8/8/8/2N1K3 w - - 0 1", true},
		{"bispos na mesma cor de casa", "2b1k3/8/8/8/8/8/8/4KB2 w - - 0 1", true},
		{"bispos em cores diferentes", "2b1k3/8/8/8/8/8/8/2B1K3 w - - 0 1", false},
		{"rei e dois cavalos", "4k3/8/8/8/8/8/8/1NN1K3 w - - 0 1", false},
		{"ainda há um peão", "4k3/7p/8/8/8/8/8/4K3 w - - 0 1", false},
		{"ainda há uma torre", "4k3/8/8/8/8/8/8/R3K3 w - - 0 1", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			game := mustGame(t, tt.fen)
			if got := game.insufficientMaterial(); got != tt.want {
				t.Errorf("insufficientMaterial() = %v, esperado %v", got, tt.want)
			}
		})
	}
}

func TestEmpatePorMaterialInsuficienteEncerraAPartida(t *testing.T) {
	// O cavalo preto de h1 é a última peça além dos reis e do bispo branco:
	// capturá-lo deixa rei e bispo contra rei.
	game := mustGame(t, "4k3/8/8/8/8/8/6B1/4K2n w - - 0 1")

	applied := mustPlay(t, game, "Bxh1")

	if applied.Outcome != DrawInsufficientMaterial {
		t.Errorf("Outcome = %v, esperado empate por material insuficiente", applied.Outcome)
	}
}

func TestRegraDos50Lances(t *testing.T) {
	// O contador entra em 99: mais um lance sem peão nem captura fecha os 100
	// meios-lances, que são os 50 lances da regra.
	game := mustGame(t, "4k3/8/8/8/8/8/R7/4K3 w - - 99 60")

	applied := mustPlay(t, game, "Ra3")

	if applied.Outcome != DrawFiftyMove {
		t.Errorf("Outcome = %v, esperado empate pela regra dos 50 lances", applied.Outcome)
	}
}

func TestCapturaZeraOContadorDos50Lances(t *testing.T) {
	game := mustGame(t, "4k3/8/8/8/8/r7/R7/4K3 w - - 99 60")

	applied := mustPlay(t, game, "Rxa3")

	if applied.Outcome == DrawFiftyMove {
		t.Error("a captura zera o contador: não pode empatar pela regra dos 50 lances")
	}
	if !strings.HasSuffix(game.FEN(), " 0 60") {
		t.Errorf("o contador deveria ter zerado, FEN = %q", game.FEN())
	}
}

func TestRepeticaoTripla(t *testing.T) {
	game := mustGame(t, "4k1n1/8/8/8/8/8/8/4K1N1 w - - 0 1")

	// Os dois lados vão e voltam com os cavalos. A posição inicial reaparece a
	// cada quatro meios-lances; na terceira vez a partida empata.
	mustPlay(t, game, "Nf3", "Nf6", "Ng1", "Ng8", "Nf3", "Nf6", "Ng1")
	if game.Outcome() != Ongoing {
		t.Fatalf("cedo demais: Outcome = %v", game.Outcome())
	}

	applied := mustPlay(t, game, "Ng8")

	if applied.Outcome != DrawThreefold {
		t.Errorf("Outcome = %v, esperado empate por repetição tripla", applied.Outcome)
	}
}

// --- histórico ---------------------------------------------------------------

func TestHistorico(t *testing.T) {
	game := NewGame()
	mustPlay(t, game, "e4", "e5", "Nf3")

	history := game.History()
	if len(history) != 3 {
		t.Fatalf("History() tem %d lances, esperado 3", len(history))
	}

	want := []string{"e4", "e5", "Nf3"}
	for i, applied := range history {
		if applied.Notation != want[i] {
			t.Errorf("lance %d = %q, esperado %q", i, applied.Notation, want[i])
		}
	}

	history[0].Notation = "mexido"
	if game.History()[0].Notation != "e4" {
		t.Error("History() deveria devolver uma cópia")
	}
}

// --- perft -------------------------------------------------------------------

// perft conta as folhas da árvore de lances legais até a profundidade dada.
//
// É o teste padrão de um gerador de lances: os números são conhecidos e batem
// só se roque, en passant, promoção, cravada e xeque estiverem todos corretos.
// Um único desses errado muda a contagem.
func perft(game *RunningGame, depth int) int {
	// A capacidade cobre a profundidade inteira: cada nível escreve os seus
	// lances logo depois dos do pai, no mesmo buffer. Sobrando capacidade, o
	// append nunca realoca — é o que faz a contagem inteira rodar sem alocar.
	return perftInto(game, make([]Move, 0, maxMovesPerPosition*perftMaxDepth), depth)
}

const (
	// O máximo conhecido de lances legais numa posição de xadrez é 218. Arredondar
	// para cima aqui só gasta memória; errar para baixo faria o buffer realocar no
	// meio da contagem e estragar a medição de alocação.
	maxMovesPerPosition = 256
	perftMaxDepth       = 8
)

func perftInto(game *RunningGame, buf []Move, depth int) int {
	if depth == 0 {
		return 1
	}

	legal := game.AppendMoves(buf)
	if depth == 1 {
		return len(legal)
	}

	total := 0
	for _, cand := range legal {
		undo := game.Apply(cand)
		// Os lances do filho vão logo depois dos deste nível: legal continua
		// válido enquanto a recursão trabalha.
		total += perftInto(game, legal[len(legal):], depth-1)
		game.Unapply(undo)
	}
	return total
}

// TestUnapplyVoltaAoMesmoEstado é o par obrigatório do perft: o perft só prova
// que a contagem bate, e uma contagem certa não prova que desfazer restaura o
// estado inteiro. Um direito de roque ou uma casa de en passant que não voltam
// dão o mesmo número e uma partida errada.
func TestUnapplyVoltaAoMesmoEstado(t *testing.T) {
	for _, tt := range posiçõesPerft {
		t.Run(tt.nome, func(t *testing.T) {
			game := mustGame(t, tt.fen)
			verificaUnapply(t, game, 3)
		})
	}
}

// verificaUnapply joga cada lance legal, desce, desfaz, e exige que a FEN
// completa — posição, vez, roques, en passant e os dois relógios — volte a ser
// exatamente a de antes.
func verificaUnapply(t *testing.T, game *RunningGame, depth int) {
	t.Helper()
	if depth == 0 {
		return
	}

	antes := game.FEN()
	for _, cand := range game.Moves() {
		undo := game.Apply(cand)
		verificaUnapply(t, game, depth-1)
		game.Unapply(undo)

		if depois := game.FEN(); depois != antes {
			t.Fatalf("Unapply de %v não restaurou a partida: %q → %q", cand, antes, depois)
		}
	}
}

// TestMovesCasaComLegalMoves amarra o caminho rápido ao caminho da notação: os
// dois têm de enxergar exatamente o mesmo conjunto de lances. Se um dia
// divergirem, o motor vai escolher um lance que MakeMove recusa.
func TestMovesCasaComLegalMoves(t *testing.T) {
	for _, tt := range posiçõesPerft {
		t.Run(tt.nome, func(t *testing.T) {
			game := mustGame(t, tt.fen)

			notações := game.LegalMoves()
			rápidos := game.Moves()
			if len(notações) != len(rápidos) {
				t.Fatalf("LegalMoves deu %d lances, Moves deu %d", len(notações), len(rápidos))
			}

			for _, cand := range rápidos {
				notação := game.Notation(cand)

				// Cada lance rápido tem de ser aceito de volta pelas regras, e
				// pousar onde o lance dizia que ia pousar.
				filho := mustGame(t, tt.fen)
				applied, err := filho.MakeMove(notação)
				if err != nil {
					t.Errorf("Notation deu %q para %v, e MakeMove recusou: %v", notação, cand, err)
					continue
				}
				if applied.From != cand.From || applied.To != cand.To || applied.Promotion != cand.Promotion {
					t.Errorf("%q virou %d→%d, o lance era %d→%d", notação, applied.From, applied.To, cand.From, cand.To)
				}
			}
		})
	}
}

func TestPerft(t *testing.T) {
	tests := []struct {
		name string
		fen  string
		want []int // want[i] é o perft da profundidade i+1
	}{
		{
			name: "posição inicial",
			fen:  InitialFEN,
			want: []int{20, 400, 8902, 197281},
		},
		{
			// Kiwipete: a posição clássica para roque, cravada e captura.
			name: "kiwipete",
			fen:  "r3k2r/p1ppqpb1/bn2pnp1/3PN3/1p2P3/2N2Q1p/PPPBBPPP/R3K2R w KQkq - 0 1",
			want: []int{48, 2039, 97862},
		},
		{
			// Final de peões e torres, cheio de en passant e xeque descoberto.
			name: "final com en passant",
			fen:  "8/2p5/3p4/KP5r/1R3p1k/8/4P1P1/8 w - - 0 1",
			want: []int{14, 191, 2812, 43238},
		},
		{
			// Posição com promoções de todo tipo.
			name: "promoções",
			fen:  "r3k2r/Pppp1ppp/1b3nbN/nP6/BBP1P3/q4N2/Pp1P2PP/R2Q1RK1 w kq - 0 1",
			want: []int{6, 264, 9467},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for depth, want := range tt.want {
				game := mustGame(t, tt.fen)
				if got := perft(game, depth+1); got != want {
					t.Errorf("perft(%d) = %d, esperado %d", depth+1, got, want)
				}
			}
		})
	}
}

// perftPelaAPI conta as mesmas folhas, mas só pela API pública: é o caminho que
// o motor teria se `internal/engine` falasse com as regras do jeito que a TUI
// fala. Existe para medir o preço desse caminho, não para validar as regras —
// quem valida é `perft`, e os dois têm de dar o mesmo número.
//
// Cada nó aqui reparseia a notação, remonta a partida inteira a partir da FEN e
// realoca o tabuleiro. É a linha de base contra a qual o caminho rápido da fase
// 1 vai ser comparado.
func perftPelaAPI(game *RunningGame, depth int) int {
	if depth == 0 {
		return 1
	}

	legal := game.LegalMoves()
	if depth == 1 {
		return len(legal)
	}

	fen := game.FEN()
	total := 0
	for _, move := range legal {
		filho, err := NewGameFromFEN(fen)
		if err != nil {
			panic(err)
		}
		if _, err := filho.MakeMove(move); err != nil {
			panic(err)
		}
		total += perftPelaAPI(filho, depth-1)
	}
	return total
}

// posiçõesPerft é o conjunto medido pelos benchmarks. A profundidade de cada uma
// foi escolhida para o nó custar alguns milissegundos: rasa demais mede o
// arranjo do benchmark, funda demais faz cada rodada levar minutos.
var posiçõesPerft = []struct {
	nome  string
	fen   string
	depth int
	nós   int
}{
	{"posição inicial", InitialFEN, 4, 197281},
	{"kiwipete", "r3k2r/p1ppqpb1/bn2pnp1/3PN3/1p2P3/2N2Q1p/PPPBBPPP/R3K2R w KQkq - 0 1", 3, 97862},
	{"final com en passant", "8/2p5/3p4/KP5r/1R3p1k/8/4P1P1/8 w - - 0 1", 4, 43238},
	{"promoções", "r3k2r/Pppp1ppp/1b3nbN/nP6/BBP1P3/q4N2/Pp1P2PP/R2Q1RK1 w kq - 0 1", 3, 9467},
}

// BenchmarkPerft é a linha de base do custo de um nó pelo caminho interno —
// candidatos mais Clone por lance. É contra este número que o caminho rápido da
// fase 1 tem de mostrar o ganho, e é por isso que ele conta nós por segundo em
// vez de só tempo por operação: o que interessa a uma busca é quantas posições
// cabem no orçamento de um lance.
//
// O resultado é verificado a cada rodada de propósito: um benchmark que fica
// rápido porque parou de contar certo não serve de linha de base nenhuma.
func BenchmarkPerft(b *testing.B) {
	for _, p := range posiçõesPerft {
		b.Run(p.nome, func(b *testing.B) {
			game, err := NewGameFromFEN(p.fen)
			if err != nil {
				b.Fatal(err)
			}

			// O buffer nasce fora do laço de propósito: numa busca ele é
			// alocado uma vez por lance pensado, não por nó. Deixá-lo dentro
			// esconderia atrás dessa única alocação a pergunta que interessa —
			// quanto custa um nó — que aqui tem de dar zero.
			buf := make([]Move, 0, maxMovesPerPosition*perftMaxDepth)

			b.ReportAllocs()
			for b.Loop() {
				if got := perftInto(game, buf, p.depth); got != p.nós {
					b.Fatalf("perft(%d) = %d, esperado %d", p.depth, got, p.nós)
				}
			}
			b.ReportMetric(float64(p.nós)*float64(b.N)/b.Elapsed().Seconds(), "nós/s")
		})
	}
}

// BenchmarkPerftPelaAPI mede o mesmo trabalho pela API pública. Roda só a
// posição inicial, e rasa, porque é ordens de grandeza mais lento: o número
// serve para justificar a existência do caminho rápido, não para acompanhar
// regressão.
func BenchmarkPerftPelaAPI(b *testing.B) {
	game, err := NewGameFromFEN(InitialFEN)
	if err != nil {
		b.Fatal(err)
	}

	const (
		depth = 3
		nós   = 8902
	)

	b.ReportAllocs()
	for b.Loop() {
		if got := perftPelaAPI(game, depth); got != nós {
			b.Fatalf("perftPelaAPI(%d) = %d, esperado %d", depth, got, nós)
		}
	}
	b.ReportMetric(float64(nós)*float64(b.N)/b.Elapsed().Seconds(), "nós/s")
}
