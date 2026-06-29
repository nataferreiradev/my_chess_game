package my_chess_game

import (
	"fmt"

	parser "github.com/nataferreiradev/chess_notation_parser"
)

type RunningGame struct {
	board              Board
	currentPlayerColor Colors
}

// NewRunningGame devolve uma partida pronta para começar, com o tabuleiro na
// posição inicial e as brancas a jogar.
func NewRunningGame() RunningGame {
	return RunningGame{
		board:              NewBoard(),
		currentPlayerColor: White,
	}
}

// MakeMove interpreta um lance em notação algébrica, aplica-o no tabuleiro e
// passa a vez ao adversário. Devolve erro se a notação for inválida ou se
// nenhuma peça do jogador da vez puder fazer o lance.
func (game *RunningGame) MakeMove(cmd string) error {
	move, err := parser.Parse(cmd)
	if err != nil {
		return err
	}

	from, err := game.resolveOrigin(move)
	if err != nil {
		return err
	}

	to := squareToIndex(move.To)
	game.board[to] = game.board[from]
	game.board[from] = GamePiece{Piece: parser.Empty}

	game.currentPlayerColor = opponent(game.currentPlayerColor)
	return nil
}

// resolveOrigin descobre de qual casa parte o lance. O parser raramente
// informa a origem (apenas a coluna, no caso de captura de peão), por isso
// procuramos entre as peças do jogador da vez qual delas consegue,
// legalmente, alcançar o destino. As pistas que o parser fornecer
// (move.From.File / move.From.Rank) são usadas para filtrar candidatas.
func (game RunningGame) resolveOrigin(move parser.Move) (int, error) {
	to := squareToIndex(move.To)

	var candidates []int
	for idx, piece := range game.board {
		if piece.Piece != move.Piece || piece.Color != game.currentPlayerColor {
			continue
		}

		file, rank := fileRank(idx)
		if move.From.File >= 0 && move.From.File != file {
			continue
		}
		if move.From.Rank >= 0 && move.From.Rank != rank {
			continue
		}

		if game.canReach(idx, to, move) {
			candidates = append(candidates, idx)
		}
	}

	switch len(candidates) {
	case 1:
		return candidates[0], nil
	case 0:
		return -1, fmt.Errorf("nenhuma peça pode fazer o lance %q", move.String())
	default:
		return -1, fmt.Errorf("lance %q é ambíguo", move.String())
	}
}

// canReach decide se a peça em "from" pode mover-se para "to" segundo as
// regras de movimento da peça e o estado atual do tabuleiro. Não verifica se o
// próprio rei fica em xeque (o parser não trata dessa legalidade).
func (game RunningGame) canReach(from, to int, move parser.Move) bool {
	if from == to {
		return false
	}

	dest := game.board[to]
	if move.Capture {
		// Captura exige peça inimiga no destino.
		if dest.Piece == parser.Empty || dest.Color == game.currentPlayerColor {
			return false
		}
	} else {
		// Sem captura o destino tem de estar vazio.
		if dest.Piece != parser.Empty {
			return false
		}
	}

	fromFile, fromRank := fileRank(from)
	toFile, toRank := fileRank(to)
	df := toFile - fromFile
	dr := toRank - fromRank

	switch move.Piece {
	case parser.Knight:
		return (abs(df) == 1 && abs(dr) == 2) || (abs(df) == 2 && abs(dr) == 1)

	case parser.King:
		return abs(df) <= 1 && abs(dr) <= 1

	case parser.Rook:
		return (df == 0 || dr == 0) && game.pathClear(from, to)

	case parser.Bishop:
		return abs(df) == abs(dr) && game.pathClear(from, to)

	case parser.Queen:
		straight := df == 0 || dr == 0
		diagonal := abs(df) == abs(dr)
		return (straight || diagonal) && game.pathClear(from, to)

	case parser.Pawn:
		return game.canPawnReach(from, fromRank, df, dr, move.Capture)
	}

	return false
}

// canPawnReach trata as regras específicas do peão: avanço de uma casa,
// avanço duplo a partir da casa inicial e captura na diagonal. En passant não
// é suportado porque o parser não o sinaliza.
func (game RunningGame) canPawnReach(from, fromRank, df, dr int, capture bool) bool {
	dir := pawnDirection(game.currentPlayerColor)
	startRank := pawnStartRank(game.currentPlayerColor)

	if capture {
		return abs(df) == 1 && dr == dir
	}

	if df != 0 {
		return false
	}
	if dr == dir {
		return true
	}
	if dr == 2*dir && fromRank == startRank {
		// A casa intermediária também precisa estar livre.
		fromFile := from % 8
		mid := indexFor(fromRank+dir, fromFile)
		return game.board[mid].Piece == parser.Empty
	}
	return false
}

// pathClear verifica se todas as casas entre "from" e "to" (exclusivas) estão
// vazias. Usado por torre, bispo e dama.
func (game RunningGame) pathClear(from, to int) bool {
	fromFile, fromRank := fileRank(from)
	toFile, toRank := fileRank(to)

	stepFile := sign(toFile - fromFile)
	stepRank := sign(toRank - fromRank)

	file, rank := fromFile+stepFile, fromRank+stepRank
	for file != toFile || rank != toRank {
		if game.board[indexFor(file, rank)].Piece != parser.Empty {
			return false
		}
		file += stepFile
		rank += stepRank
	}
	return true
}

// --- helpers de coordenada -------------------------------------------------
//
// O tabuleiro é um slice de 64 posições onde o índice 0 é a8 e o índice 63 é
// h1 (ver NewBoard). O parser usa File a=0..h=7 e Rank "1"=0.."8"=7. Logo o
// índice é (7-rank)*8 + file.

func squareToIndex(s parser.Square) int {
	return indexFor(s.Rank, s.File)
}

func indexFor(rank, file int) int {
	return (7-rank)*8 + file
}

func fileRank(idx int) (file, rank int) {
	return idx % 8, 7 - idx/8
}

func opponent(c Colors) Colors {
	if c == White {
		return Black
	}
	return White
}

func pawnDirection(c Colors) int {
	if c == White {
		return 1 // brancas avançam de rank 1 (0) para rank 8 (7)
	}
	return -1
}

func pawnStartRank(c Colors) int {
	if c == White {
		return 1 // rank "2"
	}
	return 6 // rank "7"
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

func sign(n int) int {
	switch {
	case n > 0:
		return 1
	case n < 0:
		return -1
	default:
		return 0
	}
}
