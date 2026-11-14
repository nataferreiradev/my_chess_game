package engine

import (
	"context"
	"time"

	chess "github.com/nataferreiradev/my_chess_game"
)

// Os escores são em centipeões: 100 é um peão. As duas constantes de fora da
// escala existem para o alfa-beta ter um infinito com que trabalhar e para o
// mate valer mais que qualquer vantagem material.
const (
	scoreInfinity = 1 << 20
	scoreMate     = 1 << 16
	// maxPly é o teto de profundidade da busca, inclusive a quiescência. Serve
	// de tamanho para as tabelas por ply e de trava contra recursão infinita.
	maxPly = 64
)

// search é uma busca em andamento. Nasce e morre dentro de um Best: guardar
// estado de busca no Engine tornaria o motor não reentrante.
type search struct {
	opts     Options
	deadline time.Time
	ctx      context.Context

	nodes int
	// stopped marca que o tempo acabou no meio da árvore. A partir daí os
	// escores não valem e a profundidade em curso é descartada inteira.
	stopped bool

	// killers são os dois lances que causaram corte em cada ply sem serem
	// captura, e history pontua os lances que cortaram em qualquer ply. As
	// duas tabelas só servem para ordenar; não mudam o resultado, só o tempo.
	killers [maxPly][2]string
	history map[string]int
}

// run faz o aprofundamento iterativo: busca a profundidade 1, depois 2, e assim
// por diante até o teto ou até o tempo acabar.
//
// Parece desperdício refazer a busca rasa a cada rodada, mas não é: a busca
// rasa é barata e a ordem que ela descobre faz o alfa-beta cortar muito mais na
// rodada seguinte. E é o que dá o lance sempre pronto quando o tempo acaba.
func (s *search) run(game *chess.RunningGame) Result {
	// TODO(engine): fase 3.
	return Result{}
}

// negamax é o alfa-beta em forma negamax: o escore é sempre do ponto de vista
// de quem tem a vez, e o do adversário é o mesmo com o sinal trocado.
//
// Devolve o escore da posição. ply é a distância da raiz — é ele, e não depth,
// que entra na conta do mate, para o motor preferir o mate mais curto.
func (s *search) negamax(game *chess.RunningGame, depth, alpha, beta, ply int) int {
	// TODO(engine): fase 3.
	return 0
}

// quiescence estende a busca nas posições barulhentas: nas folhas, só para de
// olhar quando não há mais captura pendente.
//
// Sem ela o motor sofre do efeito horizonte — avalia como ótima a posição em
// que acabou de capturar uma dama e ainda vai ser recapturado no lance seguinte,
// que está um passo além da profundidade.
func (s *search) quiescence(game *chess.RunningGame, alpha, beta, ply int) int {
	// TODO(engine): fase 4.
	return 0
}

// timedOut diz se a busca tem de parar agora, por tempo ou por cancelamento.
// É consultada de tantos em tantos nós, e não em todo nó: ler o relógio é caro
// perto do custo de um nó.
func (s *search) timedOut() bool {
	if s.stopped {
		return true
	}
	if s.nodes%2048 != 0 {
		return false
	}

	select {
	case <-s.ctx.Done():
		s.stopped = true
	default:
		if !s.deadline.IsZero() && time.Now().After(s.deadline) {
			s.stopped = true
		}
	}

	return s.stopped
}
