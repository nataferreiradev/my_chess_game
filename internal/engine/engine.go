// Package engine é o jogador de computador.
//
// Ele não sabe as regras: quem sabe é o package raiz `my_chess_game`. O motor
// só escolhe, entre os lances legais que as regras oferecem, o que ele acha
// melhor — e devolve a notação algébrica desse lance, do mesmo jeito que um
// humano digitaria na TUI. A sala não precisa saber se do outro lado tem gente.
//
// O motor é puro como as regras: não importa `net`, não conhece sessão nem
// sala, e não guarda estado entre um lance e outro além do que Options manda.
// Um *Engine pode ser usado por várias salas ao mesmo tempo desde que Best seja
// chamado de goroutines diferentes — ele não escreve em nada compartilhado.
package engine

import (
	"context"
	"errors"
	"time"

	chess "github.com/nataferreiradev/my_chess_game"
)

// ErrGameOver diz que não há lance a escolher: a partida já acabou.
var ErrGameOver = errors.New("a partida já acabou")

// Level é a força do computador, como ela aparece para o jogador.
//
// O nível não muda o algoritmo, só o orçamento e o ruído: o fácil pensa raso e
// erra de propósito, o difícil pensa fundo e sempre joga o melhor que achou.
// Assim existe um motor só para depurar, e não três.
type Level int

const (
	Facil Level = iota
	Medio
	Dificil
)

// String devolve o nome do nível em português, para o menu.
func (l Level) String() string {
	switch l {
	case Facil:
		return "fácil"
	case Dificil:
		return "difícil"
	default:
		return "médio"
	}
}

// Options é o orçamento de uma busca.
func (l Level) Options() Options {
	switch l {
	case Facil:
		return Options{Depth: 2, MoveTime: 300 * time.Millisecond, Slack: 90}
	case Dificil:
		return Options{Depth: 6, MoveTime: 3 * time.Second}
	default:
		return Options{Depth: 4, MoveTime: time.Second, Slack: 25}
	}
}

// Options regula quanto o motor pensa e quanto ele erra.
type Options struct {
	// Depth é a profundidade máxima do aprofundamento iterativo. Zero deixa a
	// busca ir até o tempo acabar.
	Depth int
	// MoveTime é o teto de tempo por lance. Zero não põe teto — e aí Depth
	// precisa ser maior que zero, ou a busca não termina.
	MoveTime time.Duration
	// Slack é a tolerância, em centipeões, para escolher um lance pior que o
	// melhor encontrado. É o que faz o nível fácil variar as partidas e dar
	// chance ao adversário, sem nunca escolher um lance perdedor de material
	// grosso. Zero joga sempre o melhor.
	Slack int
	// Seed fixa o sorteio do desempate. Zero sorteia do relógio; nos testes,
	// fixar deixa a partida reproduzível.
	Seed int64
}

// Result é o que a busca achou.
type Result struct {
	// Move é o lance escolhido, em notação algébrica — pronto para ir para
	// RunningGame.MakeMove.
	Move string
	// Score é a avaliação da posição depois do lance, em centipeões e do ponto
	// de vista de quem jogou: positivo é bom para ele.
	Score int
	// Mate, quando diferente de zero, é o número de lances até o mate.
	// Positivo é mate a favor de quem joga, negativo é mate contra.
	Mate int

	Depth   int
	Nodes   int
	Elapsed time.Duration
	// PV é a variante principal: a sequência que a busca espera ver jogada, em
	// notação algébrica, começando por Move. Serve para depurar a busca.
	PV []string
}

// Engine é o jogador de computador configurado.
type Engine struct {
	opts Options
}

// New devolve um motor com o orçamento dado.
func New(opts Options) *Engine {
	return &Engine{opts: opts}
}

// Best escolhe o lance do computador na posição em que a partida está.
//
// game não é alterada: a busca trabalha sobre uma cópia montada a partir da
// FEN. É por isso que Best pode ser chamada com a partida da sala sem que a
// goroutine árbitro perca a posse dela — mas a cópia também não carrega o
// histórico, então a busca não enxerga repetição tripla. Ver PROMPT_ENGINE.md,
// "a FEN não é a partida".
//
// ctx corta a busca: quando ele termina, Best devolve o melhor lance da última
// profundidade que fechou. Só devolve erro se a partida já tiver acabado.
func (e *Engine) Best(ctx context.Context, game *chess.RunningGame) (Result, error) {
	if game.Outcome().Over() {
		return Result{}, ErrGameOver
	}

	// TODO(engine): fase 3 — trocar pelo aprofundamento iterativo de search.go.
	return Result{}, errors.New("engine: busca ainda não implementada")
}
