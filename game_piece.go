package my_chess_game

import parser "github.com/nataferreiradev/chess_notation_parser"

// Color é a cor de um jogador ou de uma peça.
type Color byte

const (
	Black Color = 0
	White Color = 1
)

// String devolve o nome da cor em português, para exibição ao jogador.
func (c Color) String() string {
	if c == White {
		return "brancas"
	}
	return "pretas"
}

// GamePiece é uma peça posicionada no tabuleiro.
type GamePiece struct {
	Piece parser.Piece
	Color Color
}

// símbolos Unicode das peças, indexados pela letra da notação. As figuras
// vazadas são as brancas e as cheias são as pretas.
var (
	whiteSymbols = map[parser.Piece]rune{
		parser.King: '♔', parser.Queen: '♕', parser.Rook: '♖',
		parser.Bishop: '♗', parser.Knight: '♘', parser.Pawn: '♙',
	}
	blackSymbols = map[parser.Piece]rune{
		parser.King: '♚', parser.Queen: '♛', parser.Rook: '♜',
		parser.Bishop: '♝', parser.Knight: '♞', parser.Pawn: '♟',
	}
)

// Symbol devolve o símbolo Unicode da peça, para a TUI desenhar o tabuleiro.
// Devolve um espaço se a peça for desconhecida.
func (p GamePiece) Symbol() rune {
	symbols := blackSymbols
	if p.Color == White {
		symbols = whiteSymbols
	}

	if symbol, ok := symbols[p.Piece]; ok {
		return symbol
	}
	return ' '
}

// Name devolve o nome da peça em português.
func (p GamePiece) Name() string {
	return pieceName(p.Piece)
}

// SolidSymbol devolve sempre a figura cheia da peça, sem distinguir a cor.
//
// Existe para a TUI: muitos terminais desenham as figuras vazadas (♔♕♖) com
// largura diferente das cheias, ou simplesmente não as têm na fonte, o que
// desalinha o tabuleiro. Usar sempre a figura cheia e separar os lados pela cor
// do texto é o que funciona em mais terminais.
func (p GamePiece) SolidSymbol() rune {
	if symbol, ok := blackSymbols[p.Piece]; ok {
		return symbol
	}
	return ' '
}
