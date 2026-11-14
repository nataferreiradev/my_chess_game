package my_chess_game

import (
	"fmt"
	"strings"

	parser "github.com/nataferreiradev/chess_notation_parser"
)

// castleRights guarda os direitos de roque de uma cor. Um direito se perde
// quando o rei se move, quando a torre daquele lado se move, ou quando ela é
// capturada na casa de origem.
type castleRights struct {
	kingside  bool
	queenside bool
}

// RunningGame é uma partida em andamento.
//
// O estado interno é privado de propósito: quem observa a partida (por exemplo
// a TUI) lê por Board, PieceAt e CurrentPlayer, e só altera o jogo por MakeMove.
//
// RunningGame não é seguro para uso concorrente. A partida tem uma dona só: no
// servidor é a goroutine árbitro, e as sessões recebem os snapshots que Board e
// AppliedMove devolvem.
type RunningGame struct {
	playingColor Color
	board        Board

	castleRights [2]castleRights
	// enPassant é a casa que um peão adversário pode ocupar capturando en
	// passant, válida só no lance seguinte. Vale -1 quando não há.
	enPassant int

	halfmoveClock  int
	fullmoveNumber int

	history []AppliedMove
	// positions conta quantas vezes cada posição apareceu, para a repetição
	// tripla. A chave ignora os contadores de lance, como manda a regra.
	positions map[string]int

	outcome Outcome
}

// NewGame devolve uma partida nova, com o tabuleiro na posição inicial e as
// brancas na vez.
func NewGame() *RunningGame {
	game := &RunningGame{
		playingColor:   White,
		board:          NewBoard(),
		castleRights:   [2]castleRights{{true, true}, {true, true}},
		enPassant:      -1,
		fullmoveNumber: 1,
		positions:      make(map[string]int),
	}
	game.positions[game.positionKey()] = 1
	return game
}

// CurrentPlayer devolve a cor de quem tem a vez.
func (game *RunningGame) CurrentPlayer() Color {
	return game.playingColor
}

// Board devolve uma cópia do tabuleiro. A cópia é profunda: alterar o resultado
// (ou as peças dentro dele) não afeta a partida.
func (game *RunningGame) Board() Board {
	return game.board.Clone()
}

// PieceAt devolve a peça da casa idx (0 = a8, 63 = h1). O segundo retorno é
// false se a casa estiver vazia ou se idx estiver fora do tabuleiro.
func (game *RunningGame) PieceAt(idx int) (GamePiece, bool) {
	if !onBoard(idx) {
		return GamePiece{}, false
	}

	piece := game.board[idx]
	if piece == nil {
		return GamePiece{}, false
	}

	return *piece, true
}

// Outcome devolve a situação da partida.
func (game *RunningGame) Outcome() Outcome {
	return game.outcome
}

// InCheck diz se o rei de quem tem a vez está em xeque.
func (game *RunningGame) InCheck() bool {
	return isSquareAttacked(game.board, findKing(game.board, game.playingColor), opponent(game.playingColor))
}

// History devolve uma cópia dos lances já jogados, na ordem. É o que o servidor
// reenvia para quem reconecta no meio da partida ou entra como espectador.
func (game *RunningGame) History() []AppliedMove {
	copied := make([]AppliedMove, len(game.history))
	copy(copied, game.history)
	return copied
}

// LegalMoves devolve a notação algébrica de todos os lances legais da vez, em
// ordem indefinida. Serve para a TUI sugerir lances e para os testes.
func (game *RunningGame) LegalMoves() []string {
	if game.outcome.Over() {
		return nil
	}

	legal := game.legalMoves(game.playingColor)
	out := make([]string, 0, len(legal))
	for _, cand := range legal {
		out = append(out, san(cand, legal, false, false))
	}
	return out
}

// MakeMove interpreta um lance em notação algébrica e o aplica à partida.
// Devolve erro se a notação for inválida ou se o lance for ilegal, e nesse caso
// não altera o estado do jogo.
func (game *RunningGame) MakeMove(cmd string) (AppliedMove, error) {
	if game.outcome.Over() {
		return AppliedMove{}, fmt.Errorf("a partida já acabou: %v", game.outcome)
	}

	move, err := parser.Parse(cmd)
	if err != nil {
		return AppliedMove{}, fmt.Errorf("lance inválido %q: %w", cmd, err)
	}

	legal := game.legalMoves(game.playingColor)

	cand, err := game.resolve(move, legal)
	if err != nil {
		return AppliedMove{}, err
	}

	return game.apply(cand, legal), nil
}

// resolve encontra, entre os lances legais, o único que a notação descreve.
//
// A notação algébrica quase nunca diz a casa de origem: ela informa a peça e o
// destino, e só desambigua quando precisa. Resolver é o caminho inverso —
// procurar quais lances legais casam com o que foi escrito.
func (game *RunningGame) resolve(move parser.Move, legal []Move) (Move, error) {
	if move.Castle != parser.CastleNone {
		for _, cand := range legal {
			if cand.Castle == move.Castle {
				return cand, nil
			}
		}
		return Move{}, fmt.Errorf("o roque %v não é possível agora", move.Castle)
	}

	to, err := SquareIndex(move.To)
	if err != nil {
		return Move{}, err
	}

	var matches []Move
	for _, cand := range legal {
		if cand.Castle != parser.CastleNone || cand.Piece != move.Piece || cand.To != to {
			continue
		}
		if move.From.File >= 0 && col(cand.From) != move.From.File {
			continue
		}
		// From.Rank usa a convenção do parser (0 = linha 1), a fileira usa a do
		// tabuleiro (0 = linha 8).
		if move.From.Rank >= 0 && row(cand.From) != 7-move.From.Rank {
			continue
		}
		matches = append(matches, cand)
	}

	if len(matches) == 0 {
		return Move{}, fmt.Errorf("%s das %v alcança %s", noPiece(move.Piece), game.playingColor, SquareName(to))
	}

	matches, err = filterPromotion(matches, move, to)
	if err != nil {
		return Move{}, err
	}

	if len(matches) > 1 {
		origens := make([]string, 0, len(matches))
		for _, cand := range matches {
			origens = append(origens, SquareName(cand.From))
		}
		return Move{}, fmt.Errorf("lance ambíguo: há %s em %s; diga de qual casa, como %s",
			pieceNamePlural(move.Piece), strings.Join(origens, " e "), san(matches[0], legal, false, false))
	}

	cand := matches[0]

	// O "x" da notação tem de bater com o que acontece no tabuleiro.
	if move.Capture && !cand.IsCapture() {
		return Move{}, fmt.Errorf("não há captura em %s: o lance é %s", SquareName(to), san(cand, legal, false, false))
	}
	if !move.Capture && cand.IsCapture() {
		return Move{}, fmt.Errorf("esse lance é uma captura: escreva %s", san(cand, legal, false, false))
	}

	return cand, nil
}

// filterPromotion reduz os candidatos à promoção pedida, e explica o que falta
// quando a notação não diz em que peça o peão vira.
func filterPromotion(matches []Move, move parser.Move, to int) ([]Move, error) {
	promoting := false
	for _, cand := range matches {
		if cand.Promotion != 0 {
			promoting = true
			break
		}
	}

	if !promoting {
		if move.Promotion != 0 {
			return nil, fmt.Errorf("esse lance não é uma promoção")
		}
		return matches, nil
	}

	if move.Promotion == 0 {
		return nil, fmt.Errorf("o peão chega à última linha: diga em que peça ele vira, como %s=Q", SquareName(to))
	}

	var kept []Move
	for _, cand := range matches {
		if cand.Promotion == move.Promotion {
			kept = append(kept, cand)
		}
	}
	if len(kept) == 0 {
		return nil, fmt.Errorf("não dá para promover a %s", pieceName(move.Promotion))
	}
	return kept, nil
}

// apply escreve o lance no tabuleiro e atualiza todo o estado derivado:
// direitos de roque, casa de en passant, contadores, histórico e fim de jogo.
func (game *RunningGame) apply(cand Move, legal []Move) AppliedMove {
	mover := game.playingColor

	// AppliedMove é entregue para fora e carrega ponteiro, então a peça
	// capturada é remontada aqui como um valor novo: nada que saia da partida
	// pode apontar para dentro do tabuleiro vivo.
	var captured *GamePiece
	if cand.IsCapture() {
		captured = &GamePiece{Piece: cand.Captured, Color: opponent(mover)}
	}

	// O tabuleiro e todo o estado que decide os lances seguintes são escritos
	// pelo caminho rápido: um lance legal é sempre aplicado do mesmo jeito, e
	// duas implementações disso acabariam divergindo num caso de borda —
	// promoção com captura, roque perdendo direito, en passant — que só
	// apareceria com o jogador humano olhando.
	game.Apply(cand)

	waiting := game.playingColor
	check := isSquareAttacked(game.board, findKing(game.board, waiting), mover)
	hasReply := len(game.legalMoves(waiting)) > 0

	game.positions[game.positionKey()]++
	game.outcome = game.detectOutcome(check, hasReply)

	applied := AppliedMove{
		Player:     mover,
		Piece:      cand.Piece,
		From:       cand.From,
		To:         cand.To,
		Captured:   captured,
		CapturedAt: cand.CapturedAt,
		EnPassant:  cand.EnPassant,
		Castle:     cand.Castle,
		RookFrom:   cand.RookFrom,
		RookTo:     cand.RookTo,
		Promotion:  cand.Promotion,
		Check:      check,
		Outcome:    game.outcome,
		Notation:   san(cand, legal, check, game.outcome == Checkmate),
	}

	game.history = append(game.history, applied)
	return applied
}

// detectOutcome decide a situação da partida depois do lance. O mate e o
// afogamento vêm antes dos empates por contador: não adianta contar 50 lances
// se o lance que acabou de ser jogado deu mate.
func (game *RunningGame) detectOutcome(check, hasReply bool) Outcome {
	switch {
	case !hasReply && check:
		return Checkmate
	case !hasReply:
		return Stalemate
	case game.insufficientMaterial():
		return DrawInsufficientMaterial
	case game.positions[game.positionKey()] >= 3:
		return DrawThreefold
	case game.halfmoveClock >= 100:
		return DrawFiftyMove
	}
	return Ongoing
}

// enPassantSquare devolve a casa que o peão adversário poderá ocupar capturando
// en passant, ou -1. Só o avanço duplo de peão cria essa casa, e ela vale
// apenas no lance imediatamente seguinte.
func enPassantSquare(cand Move, mover Color) int {
	if cand.Piece != parser.Pawn {
		return -1
	}
	if row(cand.From)-row(cand.To) != 2 && row(cand.To)-row(cand.From) != 2 {
		return -1
	}
	return index(row(cand.From)+pawnDirection(mover), col(cand.From))
}

// updateCastleRights derruba os direitos que o lance destrói: o rei que se
// move perde os dois, a torre que sai da casa de origem perde o seu, e a torre
// capturada na casa de origem faz o dono perder o dele.
func (game *RunningGame) updateCastleRights(cand Move) {
	if cand.Piece == parser.King {
		game.castleRights[game.playingColor] = castleRights{}
	}

	game.dropRookRight(cand.From)
	if cand.CapturedAt >= 0 {
		game.dropRookRight(cand.CapturedAt)
	}
}

// dropRookRight derruba o direito de roque ligado à casa de origem de uma torre.
func (game *RunningGame) dropRookRight(sq int) {
	switch sq {
	case 56: // a1
		game.castleRights[White].queenside = false
	case 63: // h1
		game.castleRights[White].kingside = false
	case 0: // a8
		game.castleRights[Black].queenside = false
	case 7: // h8
		game.castleRights[Black].kingside = false
	}
}

// insufficientMaterial diz se nenhum dos lados tem material para dar mate.
//
// Cobre os casos que a regra trata como empate imediato: rei contra rei, rei e
// peça menor contra rei, e rei e bispo contra rei e bispo com os dois bispos em
// casas da mesma cor. Rei e dois cavalos não entra: o mate é impossível de
// forçar, mas não é impossível.
func (game *RunningGame) insufficientMaterial() bool {
	var minors [2][]int

	for idx, piece := range game.board {
		if piece == nil || piece.Piece == parser.King {
			continue
		}
		switch piece.Piece {
		case parser.Pawn, parser.Rook, parser.Queen:
			return false
		case parser.Bishop, parser.Knight:
			minors[piece.Color] = append(minors[piece.Color], idx)
		}
	}

	white, black := len(minors[White]), len(minors[Black])
	if white+black <= 1 {
		return true
	}

	if white == 1 && black == 1 {
		w, b := minors[White][0], minors[Black][0]
		if game.board[w].Piece == parser.Bishop && game.board[b].Piece == parser.Bishop {
			return squareColor(w) == squareColor(b)
		}
	}

	return false
}

// squareColor devolve a cor da casa: 0 e 1 para as duas cores do tabuleiro.
func squareColor(idx int) int {
	return (row(idx) + col(idx)) % 2
}

// san monta a notação algébrica canônica do lance, com a desambiguação mínima
// necessária e o sufixo de xeque ou mate. legal são os lances legais da posição
// de onde o lance parte, e é o que permite saber se a desambiguação é precisa.
func san(cand Move, legal []Move, check, mate bool) string {
	var sb strings.Builder

	switch {
	case cand.Castle != parser.CastleNone:
		sb.WriteString(cand.Castle.String())

	case cand.Piece == parser.Pawn:
		if cand.IsCapture() {
			sb.WriteByte(byte('a' + col(cand.From)))
			sb.WriteByte('x')
		}
		sb.WriteString(SquareName(cand.To))
		if cand.Promotion != 0 {
			sb.WriteByte('=')
			sb.WriteByte(byte(cand.Promotion))
		}

	default:
		sb.WriteByte(byte(cand.Piece))
		sb.WriteString(disambiguation(cand, legal))
		if cand.IsCapture() {
			sb.WriteByte('x')
		}
		sb.WriteString(SquareName(cand.To))
	}

	if mate {
		sb.WriteByte('#')
	} else if check {
		sb.WriteByte('+')
	}

	return sb.String()
}

// disambiguation devolve o mínimo que distingue este lance dos outros lances
// legais da mesma peça para a mesma casa: a coluna, ou a linha, ou a casa
// inteira. Devolve string vazia quando não há outro lance concorrente.
func disambiguation(cand Move, legal []Move) string {
	sameFile, sameRank, disputed := false, false, false

	for _, other := range legal {
		if other.From == cand.From || other.Castle != parser.CastleNone {
			continue
		}
		if other.Piece != cand.Piece || other.To != cand.To {
			continue
		}
		disputed = true
		if col(other.From) == col(cand.From) {
			sameFile = true
		}
		if row(other.From) == row(cand.From) {
			sameRank = true
		}
	}

	switch {
	case !disputed:
		return ""
	case !sameFile:
		return string(byte('a' + col(cand.From)))
	case !sameRank:
		return string(byte('8' - row(cand.From)))
	default:
		return SquareName(cand.From)
	}
}

// As mensagens de erro são lidas pelo jogador, então concordam em gênero e
// número: torre e dama são femininas, o resto é masculino.

// noPiece devolve "nenhum cavalo" ou "nenhuma torre", conforme a peça.
func noPiece(p parser.Piece) string {
	if feminine(p) {
		return "nenhuma " + pieceName(p)
	}
	return "nenhum " + pieceName(p)
}

// pieceNamePlural devolve o nome da peça no plural.
func pieceNamePlural(p parser.Piece) string {
	switch p {
	case parser.Pawn:
		return "peões"
	case parser.King:
		return "reis"
	}
	return pieceName(p) + "s"
}

func feminine(p parser.Piece) bool {
	return p == parser.Rook || p == parser.Queen
}

// pieceName devolve o nome da peça em português, para as mensagens de erro.
func pieceName(p parser.Piece) string {
	switch p {
	case parser.Pawn:
		return "peão"
	case parser.Knight:
		return "cavalo"
	case parser.Bishop:
		return "bispo"
	case parser.Rook:
		return "torre"
	case parser.Queen:
		return "dama"
	case parser.King:
		return "rei"
	}
	return "peça"
}
