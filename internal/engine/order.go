package engine

// orderMoves põe na frente os lances com mais chance de causar corte no
// alfa-beta.
//
// A ordem não muda o resultado da busca, só o tempo dela — mas muda tanto que é
// a diferença entre olhar a profundidade 3 e a 6 no mesmo segundo. O alfa-beta
// só corta bem quando o melhor lance vem cedo.
//
// A ordem é: o lance da variante principal da rodada anterior, as capturas por
// MVV-LVA (a vítima mais gorda com o agressor mais magro: PxQ antes de QxP), os
// killers do ply, e o resto por history.
//
// TODO(engine): fase 3 — a assinatura muda junto com a representação de lance
// (ver PROMPT_ENGINE.md, fase 1): ordenar string de notação obriga a reparsear.
func (s *search) orderMoves(moves []string, pv string, ply int) []string {
	return moves
}
