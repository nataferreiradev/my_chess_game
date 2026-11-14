package room

import (
	"testing"
	"time"

	chess "github.com/nataferreiradev/my_chess_game"
)

// espera é quanto um teste aguarda uma atualização do árbitro antes de desistir.
const espera = 2 * time.Second

// spy é um Listener de teste: guarda tudo que a sala manda.
type spy struct {
	updates chan Update
}

func newSpy() *spy {
	// O buffer é generoso de propósito: um Listener não pode bloquear o árbitro.
	return &spy{updates: make(chan Update, 256)}
}

func (s *spy) listen(u Update) { s.updates <- u }

func (s *spy) next(t *testing.T) Update {
	t.Helper()
	select {
	case u := <-s.updates:
		return u
	case <-time.After(espera):
		t.Fatal("a sala não mandou atualização nenhuma")
		return nil
	}
}

// snapshot devolve o próximo Snapshot, pulando os avisos pelo caminho.
func (s *spy) snapshot(t *testing.T) Snapshot {
	t.Helper()
	for {
		if snap, ok := s.next(t).(Snapshot); ok {
			return snap
		}
	}
}

// notice devolve o próximo Notice, pulando os snapshots pelo caminho.
func (s *spy) notice(t *testing.T) Notice {
	t.Helper()
	for {
		if notice, ok := s.next(t).(Notice); ok {
			return notice
		}
	}
}

// settle espera a sala terminar tudo que já foi enfileirado.
//
// Os comandos da sala são assíncronos: sem essa barreira, drenar um spy é uma
// corrida contra o árbitro. Como a fila é FIFO, quando este comando roda todos
// os anteriores já rodaram e já entregaram o que tinham para entregar.
func settle(t *testing.T, r *Room) {
	t.Helper()

	done := make(chan struct{})
	if !r.do(func(*table) { close(done) }) {
		return // a sala já fechou
	}

	select {
	case <-done:
	case <-time.After(espera):
		t.Fatal("o árbitro não respondeu")
	}
}

func (s *spy) drain() {
	for {
		select {
		case <-s.updates:
		default:
			return
		}
	}
}

// sala monta uma sala com os dois jogadores já conectados e o estado inicial já
// consumido, para os testes começarem do zero.
func sala(t *testing.T) (*Room, *spy, *spy) {
	t.Helper()

	r := New("teste", "sala de teste", "branco", "Ana")
	t.Cleanup(r.Close)

	if err := r.Sit("preto", "Bia"); err != nil {
		t.Fatalf("Bia não conseguiu sentar: %v", err)
	}

	white, black := newSpy(), newSpy()
	r.Join("branco", "Ana", white.listen)
	r.Join("preto", "Bia", black.listen)

	settle(t, r)
	white.drain()
	black.drain()

	return r, white, black
}

func TestJoinEntregaOEstadoDaPartida(t *testing.T) {
	r := New("teste", "sala de teste", "branco", "Ana")
	defer r.Close()

	if err := r.Sit("preto", "Bia"); err != nil {
		t.Fatalf("Bia não conseguiu sentar: %v", err)
	}

	white := newSpy()
	r.Join("branco", "Ana", white.listen)

	snap := white.snapshot(t)

	if snap.YourColor != chess.White {
		t.Errorf("YourColor = %v, esperado brancas", snap.YourColor)
	}
	if snap.Spectator {
		t.Error("quem tem cadeira na sala não é espectador")
	}
	if snap.Turn != chess.White {
		t.Errorf("Turn = %v, a partida começa com as brancas", snap.Turn)
	}
	if len(snap.Board) != chess.BoardSize {
		t.Errorf("Board tem %d casas, esperado %d", len(snap.Board), chess.BoardSize)
	}
	if snap.OpponentOnline {
		t.Error("o adversário ainda não entrou")
	}
}

func TestLanceEhPropagadoParaOsDois(t *testing.T) {
	r, white, black := sala(t)

	r.Move("branco", "e4")

	for name, s := range map[string]*spy{"brancas": white, "pretas": black} {
		snap := s.snapshot(t)
		if snap.LastMove == nil {
			t.Fatalf("%s: LastMove deveria trazer o lance", name)
		}
		if snap.LastMove.Notation != "e4" {
			t.Errorf("%s: LastMove = %q, esperado \"e4\"", name, snap.LastMove.Notation)
		}
		if snap.Turn != chess.Black {
			t.Errorf("%s: Turn = %v, a vez passou para as pretas", name, snap.Turn)
		}
		if len(snap.History) != 1 {
			t.Errorf("%s: History tem %d lances, esperado 1", name, len(snap.History))
		}
	}
}

func TestLanceIlegalViraAvisoSoParaQuemJogou(t *testing.T) {
	r, white, black := sala(t)

	r.Move("branco", "Ra3")

	notice := white.notice(t)
	if !notice.Error {
		t.Error("um lance recusado é um aviso de erro")
	}
	if notice.Text == "" {
		t.Error("o aviso precisa explicar o que houve")
	}

	// O adversário não recebe nada, e a partida não andou.
	r.Move("branco", "e4")
	snap := black.snapshot(t)
	if snap.LastMove == nil || snap.LastMove.Notation != "e4" {
		t.Errorf("as pretas deveriam ver só o e4, viram %+v", snap.LastMove)
	}
	if len(snap.History) != 1 {
		t.Errorf("o lance recusado não pode entrar no histórico, que tem %d lances", len(snap.History))
	}
}

func TestNaoDaParaJogarForaDaSuaVez(t *testing.T) {
	r, _, black := sala(t)

	r.Move("preto", "e5")

	notice := black.notice(t)
	if !notice.Error || notice.Text != "não é a sua vez." {
		t.Errorf("Notice = %+v, esperado aviso de vez", notice)
	}
}

func TestEspectadorAssisteMasNaoJoga(t *testing.T) {
	r, _, _ := sala(t)

	watcher := newSpy()
	r.Join("plateia", "Caio", watcher.listen)

	snap := watcher.snapshot(t)
	if !snap.Spectator {
		t.Error("quem entra numa sala cheia entra como espectador")
	}

	r.Move("plateia", "e4")
	notice := watcher.notice(t)
	if !notice.Error {
		t.Errorf("o espectador não joga, Notice = %+v", notice)
	}
}

func TestDesconexaoNaoEncerraAPartida(t *testing.T) {
	r, white, black := sala(t)

	r.Move("branco", "e4")
	settle(t, r)
	white.drain()
	black.drain()

	r.Leave("preto")

	snap := white.snapshot(t)
	if snap.OpponentOnline {
		t.Error("OpponentOnline deveria virar false quando o adversário cai")
	}
	if snap.Outcome.Over() {
		t.Errorf("a partida continua de pé, Outcome = %v", snap.Outcome)
	}
}

func TestReconexaoRecebeOHistoricoInteiro(t *testing.T) {
	r, white, black := sala(t)

	r.Move("branco", "e4")
	r.Move("preto", "e5")
	r.Move("branco", "Nf3")
	settle(t, r)
	white.drain()
	black.drain()

	r.Leave("preto")

	// A mesma identidade volta com uma sessão nova.
	voltou := newSpy()
	r.Join("preto", "Bia", voltou.listen)

	snap := voltou.snapshot(t)

	if snap.YourColor != chess.Black {
		t.Errorf("YourColor = %v, quem volta reassume a sua cor", snap.YourColor)
	}
	if len(snap.History) != 3 {
		t.Fatalf("History tem %d lances, esperado 3", len(snap.History))
	}
	for i, want := range []string{"e4", "e5", "Nf3"} {
		if snap.History[i].Notation != want {
			t.Errorf("History[%d] = %q, esperado %q", i, snap.History[i].Notation, want)
		}
	}
	if snap.Turn != chess.Black {
		t.Errorf("Turn = %v, a vez continua sendo das pretas", snap.Turn)
	}
}

func TestMateChegaNosDoisLados(t *testing.T) {
	r, white, black := sala(t)

	for _, move := range []struct {
		id   PlayerID
		move string
	}{
		{"branco", "f3"}, {"preto", "e5"}, {"branco", "g4"}, {"preto", "Qh4"},
	} {
		r.Move(move.id, move.move)
	}

	for name, s := range map[string]*spy{"brancas": white, "pretas": black} {
		var snap Snapshot
		for !snap.Outcome.Over() {
			snap = s.snapshot(t)
		}
		if snap.Outcome != chess.Checkmate {
			t.Errorf("%s: Outcome = %v, esperado xeque-mate", name, snap.Outcome)
		}
		if snap.Winner != chess.Black {
			t.Errorf("%s: Winner = %v, quem deu o mate foram as pretas", name, snap.Winner)
		}
	}
}

func TestPartidaAcabadaRecusaLances(t *testing.T) {
	r, white, _ := sala(t)

	for _, move := range []struct {
		id   PlayerID
		move string
	}{
		{"branco", "f3"}, {"preto", "e5"}, {"branco", "g4"}, {"preto", "Qh4"},
	} {
		r.Move(move.id, move.move)
	}
	settle(t, r)
	white.drain()

	r.Move("branco", "Nc3")
	notice := white.notice(t)
	if !notice.Error {
		t.Errorf("Notice = %+v, esperado aviso de partida encerrada", notice)
	}
}

func TestDesistencia(t *testing.T) {
	r, white, black := sala(t)

	r.Resign("branco")

	// Cada lado recebe o aviso e o estado final.
	for name, s := range map[string]*spy{"brancas": white, "pretas": black} {
		notice := s.notice(t)
		if notice.Error {
			t.Errorf("%s: a desistência é um recado, não um erro", name)
		}

		snap := s.snapshot(t)
		if !snap.Resigned {
			t.Errorf("%s: Resigned deveria ser true", name)
		}
		if snap.Winner != chess.Black {
			t.Errorf("%s: Winner = %v, esperado pretas", name, snap.Winner)
		}
	}

	// E ninguém joga mais.
	settle(t, r)
	white.drain()
	r.Move("branco", "e4")
	if notice := white.notice(t); !notice.Error {
		t.Errorf("Notice = %+v, a partida acabou", notice)
	}
}

func TestSnapshotNaoCompartilhaOTabuleiroDaPartida(t *testing.T) {
	r, white, _ := sala(t)

	r.Move("branco", "e4")
	snap := white.snapshot(t)

	// A sessão mexe no que recebeu; a partida não pode sentir.
	snap.Board[0] = nil
	snap.History[0].Notation = "mexido"

	settle(t, r)
	white.drain()
	r.Move("preto", "e5")
	depois := white.snapshot(t)

	if depois.Board[0] == nil {
		t.Error("mexer no tabuleiro recebido apagou uma peça da partida")
	}
	if depois.History[0].Notation != "e4" {
		t.Errorf("mexer no histórico recebido mudou a partida: %q", depois.History[0].Notation)
	}
}

func TestSalaFechaQuandoAPartidaAcabaETodosSaem(t *testing.T) {
	r, _, _ := sala(t)

	r.Resign("branco")
	r.Leave("branco")
	r.Leave("preto")

	select {
	case <-r.Closed():
	case <-time.After(espera):
		t.Error("a sala deveria fechar sozinha depois da partida acabada e vazia")
	}
}

func TestComandosDepoisDeFecharNaoTravam(t *testing.T) {
	r := New("teste", "sala de teste", "branco", "Ana")
	r.Close()

	// Uma sessão que ainda não percebeu que a sala fechou não pode ficar presa.
	done := make(chan struct{})
	go func() {
		defer close(done)
		r.Join("branco", "Ana", func(Update) {})
		_ = r.Sit("preto", "Bia")
		r.Move("branco", "e4")
		r.Leave("branco")
		r.Resign("branco")
	}()

	select {
	case <-done:
	case <-time.After(espera):
		t.Fatal("comandos numa sala fechada travaram")
	}
}

func TestSalaNasceEsperandoAdversario(t *testing.T) {
	r := New("teste", "sala de teste", "branco", "Ana")
	defer r.Close()

	white := newSpy()
	r.Join("branco", "Ana", white.listen)

	snap := white.snapshot(t)
	if snap.WhiteName != "Ana" {
		t.Errorf("WhiteName = %q, esperado Ana", snap.WhiteName)
	}
	if snap.BlackName != "" {
		t.Errorf("BlackName = %q, a cadeira ainda está vazia", snap.BlackName)
	}
	if snap.Room != "teste" || snap.Title != "sala de teste" {
		t.Errorf("a sala veio como %q/%q", snap.Room, snap.Title)
	}

	// Sem adversário não há partida: o lance é recusado com aviso, e a sala
	// segue de pé esperando alguém sentar.
	r.Move("branco", "e4")
	if notice := white.notice(t); !notice.Error {
		t.Errorf("esperado um aviso de erro, veio %+v", notice)
	}
}

func TestSitPreencheACadeiraUmaVezSo(t *testing.T) {
	r := New("teste", "sala de teste", "branco", "Ana")
	defer r.Close()

	if err := r.Sit("preto", "Bia"); err != nil {
		t.Fatalf("a cadeira estava livre: %v", err)
	}
	if err := r.Sit("caio", "Caio"); err != ErrRoomFull {
		t.Errorf("Sit numa sala cheia = %v, esperado ErrRoomFull", err)
	}
	// Quem já tem cadeira pode sentar de novo: é o caminho de quem reconecta.
	if err := r.Sit("preto", "Bia"); err != nil {
		t.Errorf("sentar de novo devolveu %v", err)
	}

	white := newSpy()
	r.Join("branco", "Ana", white.listen)
	if snap := white.snapshot(t); snap.BlackName != "Bia" {
		t.Errorf("BlackName = %q, esperado Bia", snap.BlackName)
	}
}

func TestSalaSemAdversarioFechaQuandoOAnfitriaoSai(t *testing.T) {
	r := New("teste", "sala de teste", "branco", "Ana")
	defer r.Close()

	r.Join("branco", "Ana", func(Update) {})
	r.Leave("branco")

	select {
	case <-r.Closed():
	case <-time.After(espera):
		t.Error("uma sala anunciada e sem ninguém não pode continuar aberta")
	}
}
