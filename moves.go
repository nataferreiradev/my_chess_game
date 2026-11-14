package my_chess_game

import parser "github.com/nataferreiradev/chess_notation_parser"

// Deslocamentos em (fileira, coluna). Trabalhar em fileira/coluna em vez de
// somar direto no índice evita o clássico bug de "dar a volta" na borda do
// tabuleiro: index devolve -1 quando a casa sai do tabuleiro.
var (
	knightSteps = [8][2]int{{-2, -1}, {-2, 1}, {-1, -2}, {-1, 2}, {1, -2}, {1, 2}, {2, -1}, {2, 1}}
	kingSteps   = [8][2]int{{-1, -1}, {-1, 0}, {-1, 1}, {0, -1}, {0, 1}, {1, -1}, {1, 0}, {1, 1}}
	rookDirs    = [4][2]int{{-1, 0}, {1, 0}, {0, -1}, {0, 1}}
	bishopDirs  = [4][2]int{{-1, -1}, {-1, 1}, {1, -1}, {1, 1}}
)

// opponent devolve a cor adversária.
func opponent(c Color) Color {
	if c == White {
		return Black
	}
	return White
}

// pawnDirection devolve quanto o peão da cor anda em fileiras por lance.
//
// É -1 para as brancas: como o índice 0 é a8, a linha 8 fica no começo do
// slice e avançar para a linha 8 significa ir para índices menores.
func pawnDirection(c Color) int {
	if c == White {
		return -1
	}
	return 1
}

// pawnStartRow é a fileira de onde o peão ainda pode dar o avanço duplo.
func pawnStartRow(c Color) int {
	if c == White {
		return 6
	}
	return 1
}

// pawnPromotionRow é a fileira onde o peão promove.
func pawnPromotionRow(c Color) int {
	if c == White {
		return 0
	}
	return 7
}

// promotionChoices são as peças em que um peão pode virar.
var promotionChoices = [4]parser.Piece{parser.Queen, parser.Rook, parser.Bishop, parser.Knight}

// Clone devolve uma cópia profunda do tabuleiro: alterar o resultado, ou as
// peças dentro dele, não afeta o original.
func (b Board) Clone() Board {
	copied := make(Board, len(b))
	for i, piece := range b {
		if piece == nil {
			continue
		}
		clone := *piece
		copied[i] = &clone
	}
	return copied
}

// findKing devolve a casa do rei da cor, ou -1 se não houver rei no tabuleiro.
func findKing(b Board, c Color) int {
	for idx, piece := range b {
		if piece != nil && piece.Color == c && piece.Piece == parser.King {
			return idx
		}
	}
	return -1
}

// isSquareAttacked diz se a casa idx é atacada por alguma peça da cor by.
//
// É a base de tudo que envolve xeque: legalidade de lance, roque, mate e
// afogamento. Olha a partir da casa atacada para fora, em vez de gerar todos os
// lances do adversário.
func isSquareAttacked(b Board, idx int, by Color) bool {
	if !onBoard(idx) {
		return false
	}

	r, c := row(idx), col(idx)

	for _, s := range knightSteps {
		if at := index(r+s[0], c+s[1]); at >= 0 {
			if p := b[at]; p != nil && p.Color == by && p.Piece == parser.Knight {
				return true
			}
		}
	}

	for _, s := range kingSteps {
		if at := index(r+s[0], c+s[1]); at >= 0 {
			if p := b[at]; p != nil && p.Color == by && p.Piece == parser.King {
				return true
			}
		}
	}

	// Um peão que ataca idx está uma fileira atrás do próprio avanço, na
	// diagonal: se ele anda para dir, ele veio de -dir.
	dir := pawnDirection(by)
	for _, dc := range [2]int{-1, 1} {
		if at := index(r-dir, c+dc); at >= 0 {
			if p := b[at]; p != nil && p.Color == by && p.Piece == parser.Pawn {
				return true
			}
		}
	}

	return attackedAlongRays(b, r, c, by, rookDirs[:], parser.Rook) ||
		attackedAlongRays(b, r, c, by, bishopDirs[:], parser.Bishop)
}

// attackedAlongRays caminha cada direção até esbarrar numa peça e diz se essa
// peça é um atacante de longo alcance da cor by. A dama entra nas duas buscas.
func attackedAlongRays(b Board, r, c int, by Color, dirs [][2]int, slider parser.Piece) bool {
	for _, d := range dirs {
		for step := 1; ; step++ {
			at := index(r+d[0]*step, c+d[1]*step)
			if at < 0 {
				break
			}
			piece := b[at]
			if piece == nil {
				continue
			}
			if piece.Color == by && (piece.Piece == slider || piece.Piece == parser.Queen) {
				return true
			}
			// Qualquer outra peça bloqueia o caminho.
			break
		}
	}
	return false
}

// newCandidate monta um lance simples, já com os campos "sem valor" em -1.
func newCandidate(from, to int, piece parser.Piece) Move {
	return Move{
		From:       from,
		To:         to,
		Piece:      piece,
		CapturedAt: -1,
		RookFrom:   -1,
		RookTo:     -1,
	}
}

// pseudoMoves gera os lances da cor sem conferir se deixam o próprio rei em
// xeque. O roque é a exceção: ele já sai daqui totalmente validado, porque suas
// condições de xeque fazem parte da própria regra do lance.
func (game *RunningGame) pseudoMoves(out []Move, c Color) []Move {
	for idx, piece := range game.board {
		if piece == nil || piece.Color != c {
			continue
		}

		switch piece.Piece {
		case parser.Pawn:
			out = game.pawnMoves(out, idx, piece)
		case parser.Knight:
			out = game.stepMoves(out, idx, piece, knightSteps[:])
		case parser.King:
			out = game.stepMoves(out, idx, piece, kingSteps[:])
		case parser.Bishop:
			out = game.rayMoves(out, idx, piece, bishopDirs[:])
		case parser.Rook:
			out = game.rayMoves(out, idx, piece, rookDirs[:])
		case parser.Queen:
			out = game.rayMoves(out, idx, piece, rookDirs[:])
			out = game.rayMoves(out, idx, piece, bishopDirs[:])
		}
	}

	return game.castleMoves(out, c)
}

// stepMoves gera os lances de cavalo e rei: um passo por deslocamento, parando
// em peça própria e capturando peça adversária.
func (game *RunningGame) stepMoves(out []Move, from int, piece *GamePiece, steps [][2]int) []Move {
	r, c := row(from), col(from)

	for _, s := range steps {
		to := index(r+s[0], c+s[1])
		if to < 0 {
			continue
		}

		target := game.board[to]
		if target != nil && target.Color == piece.Color {
			continue
		}

		cand := newCandidate(from, to, piece.Piece)
		if target != nil {
			cand.Captured = target.Piece
			cand.CapturedAt = to
		}
		out = append(out, cand)
	}

	return out
}

// rayMoves gera os lances de bispo, torre e dama, parando no primeiro bloqueio.
func (game *RunningGame) rayMoves(out []Move, from int, piece *GamePiece, dirs [][2]int) []Move {
	r, c := row(from), col(from)

	for _, d := range dirs {
		for step := 1; ; step++ {
			to := index(r+d[0]*step, c+d[1]*step)
			if to < 0 {
				break
			}

			target := game.board[to]
			if target != nil && target.Color == piece.Color {
				break
			}

			cand := newCandidate(from, to, piece.Piece)
			if target != nil {
				cand.Captured = target.Piece
				cand.CapturedAt = to
				out = append(out, cand)
				break
			}
			out = append(out, cand)
		}
	}

	return out
}

// pawnMoves gera avanço simples, avanço duplo, captura na diagonal e en
// passant, expandindo cada lance que chega à última fileira nas quatro
// promoções possíveis.
func (game *RunningGame) pawnMoves(out []Move, from int, piece *GamePiece) []Move {
	b := game.board
	dir := pawnDirection(piece.Color)
	r, c := row(from), col(from)

	if to := index(r+dir, c); to >= 0 && b[to] == nil {
		out = appendPawnMove(out, newCandidate(from, to, parser.Pawn), piece.Color)

		// O avanço duplo só sai da fileira inicial e só com as duas casas livres.
		if r == pawnStartRow(piece.Color) {
			if to2 := index(r+2*dir, c); to2 >= 0 && b[to2] == nil {
				out = append(out, newCandidate(from, to2, parser.Pawn))
			}
		}
	}

	for _, dc := range [2]int{-1, 1} {
		to := index(r+dir, c+dc)
		if to < 0 {
			continue
		}

		target := b[to]
		switch {
		case target != nil && target.Color != piece.Color:
			cand := newCandidate(from, to, parser.Pawn)
			cand.Captured = target.Piece
			cand.CapturedAt = to
			out = appendPawnMove(out, cand, piece.Color)

		case target == nil && to == game.enPassant:
			// No en passant o peão capturado não está na casa de destino: ele
			// ficou ao lado de quem captura.
			at := index(r, c+dc)
			victim := b[at]
			if victim == nil || victim.Color == piece.Color || victim.Piece != parser.Pawn {
				continue
			}
			cand := newCandidate(from, to, parser.Pawn)
			cand.Captured = victim.Piece
			cand.CapturedAt = at
			cand.EnPassant = true
			out = appendPawnMove(out, cand, piece.Color)
		}
	}

	return out
}

// appendPawnMove acrescenta o lance, desdobrando-o nas quatro promoções quando
// o destino é a última fileira.
func appendPawnMove(out []Move, cand Move, c Color) []Move {
	if row(cand.To) != pawnPromotionRow(c) {
		return append(out, cand)
	}

	for _, promo := range promotionChoices {
		promoted := cand
		promoted.Promotion = promo
		out = append(out, promoted)
	}
	return out
}

// castleMoves gera os roques disponíveis, já validados por inteiro: direitos
// preservados, torre no lugar, caminho livre, e o rei nem em xeque, nem
// atravessando casa atacada, nem caindo em uma.
func (game *RunningGame) castleMoves(out []Move, c Color) []Move {
	rights := game.castleRights[c]
	if !rights.kingside && !rights.queenside {
		return out
	}

	kingFrom := 60 // e1
	if c == Black {
		kingFrom = 4 // e8
	}

	king := game.board[kingFrom]
	if king == nil || king.Color != c || king.Piece != parser.King {
		return out
	}
	if isSquareAttacked(game.board, kingFrom, opponent(c)) {
		return out
	}

	if rights.kingside {
		out = game.tryCastle(out, c, kingFrom, kingFrom+2, kingFrom+3, kingFrom+1,
			parser.CastleKingside, []int{kingFrom + 1, kingFrom + 2})
	}
	if rights.queenside {
		out = game.tryCastle(out, c, kingFrom, kingFrom-2, kingFrom-4, kingFrom-1,
			parser.CastleQueenside, []int{kingFrom - 1, kingFrom - 2, kingFrom - 3})
	}

	return out
}

func (game *RunningGame) tryCastle(out []Move, c Color, kingFrom, kingTo, rookFrom, rookTo int, side parser.CastleSide, mustBeEmpty []int) []Move {
	b := game.board

	rook := b[rookFrom]
	if rook == nil || rook.Color != c || rook.Piece != parser.Rook {
		return out
	}

	for _, sq := range mustBeEmpty {
		if b[sq] != nil {
			return out
		}
	}

	// O rei não pode atravessar casa atacada nem parar numa. A casa vizinha da
	// torre no roque grande pode estar atacada: o rei não passa por ela.
	step := 1
	if kingTo < kingFrom {
		step = -1
	}
	for sq := kingFrom + step; ; sq += step {
		if isSquareAttacked(b, sq, opponent(c)) {
			return out
		}
		if sq == kingTo {
			break
		}
	}

	cand := newCandidate(kingFrom, kingTo, parser.King)
	cand.Castle = side
	cand.RookFrom = rookFrom
	cand.RookTo = rookTo
	return append(out, cand)
}

// applyCandidate escreve o lance no tabuleiro e devolve o ponteiro da peça
// capturada, que é o que unapplyCandidate precisa para desfazer. Não valida
// nada: quem chama já resolveu a legalidade.
//
// A peça movida é reposicionada pelo ponteiro, não copiada. A cópia custava uma
// alocação por lance — o que numa busca de milhões de nós é o grosso do tempo —
// e não protegia ninguém: nada fora da partida enxerga estes ponteiros, porque
// Board clona e PieceAt devolve valor.
func applyCandidate(b Board, cand Move) *GamePiece {
	// A peça capturada sai primeiro: no en passant ela não está no destino, e
	// apagar só o destino a deixaria no tabuleiro.
	var captured *GamePiece
	if cand.CapturedAt >= 0 {
		captured = b[cand.CapturedAt]
		b[cand.CapturedAt] = nil
	}

	moved := b[cand.From]
	b[cand.From] = nil
	if cand.Promotion != 0 {
		moved.Piece = cand.Promotion
	}
	b[cand.To] = moved

	if cand.Castle != parser.CastleNone {
		b[cand.RookTo] = b[cand.RookFrom]
		b[cand.RookFrom] = nil
	}

	return captured
}

// unapplyCandidate desfaz exatamente o que applyCandidate fez, com a peça
// capturada que ela devolveu. A ordem é a inversa de propósito: numa captura
// comum CapturedAt é igual a To, e repor a vítima antes de esvaziar o destino
// apagaria a vítima de novo.
func unapplyCandidate(b Board, cand Move, captured *GamePiece) {
	moved := b[cand.To]
	b[cand.To] = nil
	if cand.Promotion != 0 {
		moved.Piece = parser.Pawn
	}
	b[cand.From] = moved

	if cand.CapturedAt >= 0 {
		b[cand.CapturedAt] = captured
	}

	if cand.Castle != parser.CastleNone {
		b[cand.RookFrom] = b[cand.RookTo]
		b[cand.RookTo] = nil
	}
}

// legalMoves devolve os lances da cor que não deixam o próprio rei atacado.
func (game *RunningGame) legalMoves(c Color) []Move {
	return game.appendLegalMoves(nil, c)
}

// appendLegalMoves gera os lances legais da cor no fim de dst e devolve só os
// que gerou. É o gerador de verdade; legalMoves e Moves são conveniências em
// cima dele.
//
// A legalidade é decidida jogando o lance no tabuleiro da partida e desfazendo,
// e não numa cópia. Clonar o tabuleiro por lance candidato custava 64 casas mais
// uma alocação por peça, vezes ~35 lances, vezes cada nó da busca — era o preço
// que tornava a busca inviável.
func (game *RunningGame) appendLegalMoves(dst []Move, c Color) []Move {
	start := len(dst)
	dst = game.pseudoMoves(dst, c)

	// O rei só é procurado uma vez: se o lance não for do próprio rei, ele
	// continua onde estava, e quando for, o destino é a casa nova.
	king := findKing(game.board, c)
	enemy := opponent(c)

	kept := start
	for i := start; i < len(dst); i++ {
		cand := dst[i]

		captured := applyCandidate(game.board, cand)
		at := king
		if cand.Piece == parser.King {
			at = cand.To
		}
		legal := !isSquareAttacked(game.board, at, enemy)
		unapplyCandidate(game.board, cand, captured)

		if legal {
			dst[kept] = cand
			kept++
		}
	}

	return dst[start:kept]
}
