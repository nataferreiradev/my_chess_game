package engine

import (
	parser "github.com/nataferreiradev/chess_notation_parser"
	chess "github.com/nataferreiradev/my_chess_game"
)

// pieceValue é o valor de cada peça em centipeões, na escala clássica. O rei
// não tem valor: ele nunca é capturado, e o mate já vale scoreMate.
var pieceValue = map[parser.Piece]int{
	parser.Pawn:   100,
	parser.Knight: 320,
	parser.Bishop: 330,
	parser.Rook:   500,
	parser.Queen:  900,
}

// pieceSquare é o bônus de cada peça por casa, em centipeões, sempre do ponto
// de vista das brancas e no índice deste tabuleiro (0 = a8, 63 = h1). Para as
// pretas a tabela é lida espelhada na vertical: idx → 63-idx... quase. Ver a
// armadilha 2 do PROMPT_ENGINE.md antes de escrever o espelho.
//
// É o que ensina o motor a desenvolver o cavalo para o centro e a não deixar o
// rei passeando na abertura, sem nenhuma regra explícita sobre isso.
//
// TODO(engine): fase 2 — preencher as seis tabelas.
var pieceSquare = map[parser.Piece][64]int{}

// evaluate dá a nota da posição parada, em centipeões e do ponto de vista de
// quem tem a vez: positivo quer dizer que está bom para ele.
//
// A avaliação só olha a posição: ela nunca gera lance nem olha o futuro. Quem
// olha o futuro é a busca. Misturar as duas coisas é o jeito mais rápido de
// acabar com um motor que ninguém consegue depurar.
func evaluate(game *chess.RunningGame) int {
	// TODO(engine): fase 2 — material, tabela por casa, e o resto do
	// PROMPT_ENGINE.md.
	return 0
}
