package my_chess_game

import parser "github.com/nataferreiradev/chess_notation_parser"

// O caminho rápido de lances existe para uma coisa só: deixar o jogador de
// computador (internal/engine) percorrer milhões de posições sem passar pela
// notação.
//
// Pela API pública, cada nó de uma busca custaria um parse da notação, uma
// remontagem da partida a partir da FEN e um tabuleiro novo — medido, é mais de
// cem alocações por nó. Aqui o lance já vem resolvido (Move), é aplicado no
// próprio tabuleiro da partida (Apply) e desfeito exatamente (Unapply), sem
// alocar nada.
//
// O preço disso é que Apply **não é** MakeMove: ela não mexe no histórico, nem
// na contagem de posições para repetição tripla, nem no resultado da partida.
// Quem usa o caminho rápido está explorando posições hipotéticas, não jogando —
// e uma busca que escrevesse no histórico da sala a cada nó o destruiria. Para
// jogar de verdade, a porta continua sendo MakeMove.

// Undo carrega o que Apply destruiu, para Unapply desfazer sem clonar nada.
//
// O valor é opaco de propósito: ele só faz sentido para o Apply que o devolveu,
// e tem de ser consumido na ordem inversa (uma pilha). Desfazer fora de ordem
// deixa o tabuleiro em um estado que não existiu em nenhuma partida.
type Undo struct {
	move Move
	// captured é o ponteiro que saiu do tabuleiro. Guardá-lo, em vez de
	// remontar a peça, é o que faz Unapply não alocar.
	captured *GamePiece

	castleRights   [2]castleRights
	enPassant      int
	halfmoveClock  int
	fullmoveNumber int
}

// Move devolve o lance que este Undo desfaz.
func (u Undo) Move() Move { return u.move }

// Moves devolve os lances legais de quem tem a vez.
//
// É a conveniência: aloca um slice por chamada. Quem está numa busca usa
// AppendMoves, que escreve num buffer reaproveitado e não aloca nada.
func (game *RunningGame) Moves() []Move {
	if game.outcome.Over() {
		return nil
	}
	return game.appendLegalMoves(make([]Move, 0, 48), game.playingColor)
}

// AppendMoves gera os lances legais de quem tem a vez no fim de dst e devolve
// só os lances gerados — não dst inteiro.
//
// Devolver só o pedaço novo é o que deixa a busca empilhar os lances de todos os
// plies num buffer só: cada nível chama AppendMoves(pais[len(pais):]) e escreve
// logo depois dos lances do pai, sem tocar neles e sem alocar, desde que o
// buffer tenha nascido com capacidade para a profundidade toda.
func (game *RunningGame) AppendMoves(dst []Move) []Move {
	if game.outcome.Over() {
		return dst[len(dst):]
	}
	return game.appendLegalMoves(dst, game.playingColor)
}

// Apply aplica o lance no lugar e devolve o que Unapply precisa para desfazer.
//
// m tem de ser um lance vindo de Moves ou AppendMoves nesta mesma posição.
// Apply não valida nada: validar de novo o que o gerador acabou de gerar é o
// custo que esta função existe para não pagar.
//
// Ela atualiza o tabuleiro, a vez, os direitos de roque, a casa de en passant e
// os dois relógios — tudo que muda a lista de lances seguinte. Não atualiza
// histórico, repetição tripla nem Outcome: ver o comentário no topo do arquivo.
func (game *RunningGame) Apply(m Move) Undo {
	undo := Undo{
		move:           m,
		castleRights:   game.castleRights,
		enPassant:      game.enPassant,
		halfmoveClock:  game.halfmoveClock,
		fullmoveNumber: game.fullmoveNumber,
	}

	mover := game.playingColor

	// Os direitos de roque saem antes do lance ser escrito: updateCastleRights
	// olha a casa de origem da torre, que depois de applyCandidate já está vazia.
	game.updateCastleRights(m)

	if m.Piece == parser.Pawn || m.IsCapture() {
		game.halfmoveClock = 0
	} else {
		game.halfmoveClock++
	}
	game.enPassant = enPassantSquare(m, mover)

	undo.captured = applyCandidate(game.board, m)

	if mover == Black {
		game.fullmoveNumber++
	}
	game.playingColor = opponent(mover)

	return undo
}

// Unapply volta exatamente ao estado anterior ao Apply que devolveu u.
func (game *RunningGame) Unapply(u Undo) {
	game.playingColor = opponent(game.playingColor)
	unapplyCandidate(game.board, u.move, u.captured)

	game.castleRights = u.castleRights
	game.enPassant = u.enPassant
	game.halfmoveClock = u.halfmoveClock
	game.fullmoveNumber = u.fullmoveNumber
}

// Notation devolve a notação algébrica canônica do lance na posição atual, com
// a desambiguação mínima e o sufixo de xeque ou mate — a string que MakeMove
// aceita de volta.
//
// É cara perto de Apply: ela gera os lances legais para saber se precisa
// desambiguar e joga o lance para saber se dá xeque. Serve para a raiz da
// busca, quando o lance escolhido já foi decidido, não para dentro da árvore.
func (game *RunningGame) Notation(m Move) string {
	legal := game.legalMoves(game.playingColor)

	undo := game.Apply(m)
	waiting := game.playingColor
	check := isSquareAttacked(game.board, findKing(game.board, waiting), opponent(waiting))
	mate := check && len(game.legalMoves(waiting)) == 0
	game.Unapply(undo)

	return san(m, legal, check, mate)
}
