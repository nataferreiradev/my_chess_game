package lobby

import (
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	chess "github.com/nataferreiradev/my_chess_game"
	"github.com/nataferreiradev/my_chess_game/internal/room"
)

const espera = 2 * time.Second

// snapshotDe entra na sala como o jogador e devolve o primeiro estado que ela
// manda, que é por onde dá para conferir em que cadeira ele sentou.
func snapshotDe(t *testing.T, r *room.Room, id room.PlayerID, name string) room.Snapshot {
	t.Helper()

	updates := make(chan room.Update, 16)
	r.Join(id, name, func(u room.Update) { updates <- u })

	deadline := time.After(espera)
	for {
		select {
		case u := <-updates:
			if snap, ok := u.(room.Snapshot); ok {
				return snap
			}
		case <-deadline:
			t.Fatal("a sala não mandou o estado da partida")
			return room.Snapshot{}
		}
	}
}

func TestCriarAnunciaASalaNaLista(t *testing.T) {
	l := New()

	r := l.Create("ana", "Ana", "", "")
	defer r.Close()

	lista := l.List()
	if len(lista) != 1 {
		t.Fatalf("List() trouxe %d salas, esperado 1", len(lista))
	}

	aberta := lista[0]
	switch {
	case aberta.Code != r.ID():
		t.Errorf("Code = %q, esperado %q", aberta.Code, r.ID())
	case aberta.Host != "Ana":
		t.Errorf("Host = %q, esperado Ana", aberta.Host)
	case !aberta.Waiting:
		t.Error("uma sala recém-criada espera adversário")
	case aberta.Title != "sala de Ana":
		t.Errorf("Title = %q, esperado o nome padrão", aberta.Title)
	}
}

func TestQuemCriaJogaDeBrancasEQuemEntraDePretas(t *testing.T) {
	l := New()

	criada := l.Create("ana", "Ana", "", "")
	defer criada.Close()

	entrou, err := l.Join(criada.ID(), "bia", "Bia", "")
	if err != nil {
		t.Fatalf("Bia não conseguiu entrar: %v", err)
	}
	if entrou != criada {
		t.Fatal("Bia entrou numa sala diferente da que Ana criou")
	}

	if snap := snapshotDe(t, criada, "ana", "Ana"); snap.YourColor != chess.White {
		t.Errorf("Ana ficou com %v, quem cria joga de brancas", snap.YourColor)
	}
	if snap := snapshotDe(t, criada, "bia", "Bia"); snap.YourColor != chess.Black || snap.Spectator {
		t.Errorf("Bia ficou com %v (espectador: %v), esperado pretas jogando", snap.YourColor, snap.Spectator)
	}

	// Com as duas cadeiras ocupadas, a sala continua na lista, agora só para
	// assistir.
	if lista := l.List(); len(lista) != 1 || lista[0].Waiting || lista[0].Guest != "Bia" {
		t.Errorf("a sala cheia apareceu como %+v", lista)
	}
}

func TestTerceiroAEntrarAssiste(t *testing.T) {
	l := New()

	r := l.Create("ana", "Ana", "", "")
	defer r.Close()

	if _, err := l.Join(r.ID(), "bia", "Bia", ""); err != nil {
		t.Fatalf("Bia não conseguiu entrar: %v", err)
	}

	assistindo, err := l.Join(r.ID(), "caio", "Caio", "")
	if err != nil {
		t.Fatalf("Caio não conseguiu entrar para assistir: %v", err)
	}
	if snap := snapshotDe(t, assistindo, "caio", "Caio"); !snap.Spectator {
		t.Error("quem chega na sala cheia entra como espectador")
	}

	// Espectador não fica preso à sala: ele pode criar a partida dele.
	if l.Current("caio") != nil {
		t.Error("assistir não pode contar como estar numa partida")
	}
}

func TestSalaInexistenteDaErro(t *testing.T) {
	l := New()

	if _, err := l.Join("42", "ana", "Ana", ""); !errors.Is(err, ErrNotFound) {
		t.Errorf("Join numa sala que não existe = %v, esperado ErrNotFound", err)
	}
}

func TestQuemJaEstaNumaPartidaNaoCriaOutra(t *testing.T) {
	l := New()

	primeira := l.Create("ana", "Ana", "", "")
	defer primeira.Close()

	if segunda := l.Create("ana", "Ana", "", ""); segunda != primeira {
		t.Error("criar de novo tem de devolver a partida em andamento")
	}
	if l.Rooms() != 1 {
		t.Errorf("Rooms() = %d, esperado 1", l.Rooms())
	}
}

func TestReconexaoVoltaParaAMesmaSala(t *testing.T) {
	l := New()

	r := l.Create("ana", "Ana", "", "")
	defer r.Close()

	if _, err := l.Join(r.ID(), "bia", "Bia", ""); err != nil {
		t.Fatalf("Bia não conseguiu entrar: %v", err)
	}

	// Ana cai e volta: a identidade é a mesma chave, então a partida também.
	l.Leave("ana")

	if volta := l.Current("ana"); volta != r {
		t.Error("quem reconecta tem de voltar para a partida em andamento")
	}
	// E entrar de novo pelo código não é erro nem tira a cadeira dela.
	if volta, err := l.Join(r.ID(), "ana", "Ana", ""); err != nil || volta != r {
		t.Errorf("Join de volta = (%v, %v)", volta, err)
	}
	if snap := snapshotDe(t, r, "ana", "Ana"); snap.YourColor != chess.White || snap.Spectator {
		t.Error("Ana voltou sem a cadeira das brancas")
	}
}

func TestSalaFechadaSaiDaListaELiberaOsJogadores(t *testing.T) {
	l := New()

	r := l.Create("ana", "Ana", "", "")
	if _, err := l.Join(r.ID(), "bia", "Bia", ""); err != nil {
		t.Fatalf("Bia não conseguiu entrar: %v", err)
	}

	r.Close()

	deadline := time.Now().Add(espera)
	for time.Now().Before(deadline) {
		if l.Rooms() == 0 && l.Current("ana") == nil && l.Current("bia") == nil {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}

	t.Errorf("a sala fechada não foi recolhida: %d abertas, ana=%v", l.Rooms(), l.Current("ana"))
}

func TestListaTrazAsQueEsperamPrimeiro(t *testing.T) {
	l := New()

	cheia := l.Create("ana", "Ana", "", "")
	defer cheia.Close()
	if _, err := l.Join(cheia.ID(), "bia", "Bia", ""); err != nil {
		t.Fatalf("Bia não conseguiu entrar: %v", err)
	}

	vazia := l.Create("caio", "Caio", "duelo", "")
	defer vazia.Close()

	lista := l.List()
	if len(lista) != 2 {
		t.Fatalf("List() trouxe %d salas, esperado 2", len(lista))
	}
	if !lista[0].Waiting || lista[0].Code != vazia.ID() {
		t.Errorf("a sala que espera adversário tem de vir antes: %+v", lista)
	}
	if lista[0].Title != "duelo" {
		t.Errorf("Title = %q, esperado o nome escolhido", lista[0].Title)
	}
}

func TestEntradasConcorrentesNaoDuplicamCadeira(t *testing.T) {
	l := New()

	r := l.Create("ana", "Ana", "", "")
	defer r.Close()

	// Muita gente disputando a única cadeira vaga: exatamente uma senta, e as
	// outras entram como espectadoras, sem erro e sem corrida.
	const gente = 16
	var wg sync.WaitGroup
	var mu sync.Mutex
	var jogando int

	for i := range gente {
		wg.Add(1)
		go func() {
			defer wg.Done()

			id := room.PlayerID(fmt.Sprintf("jogador-%d", i))
			sala, err := l.Join(r.ID(), id, string(id), "")
			if err != nil || sala != r {
				t.Errorf("%s: Join = (%v, %v)", id, sala, err)
				return
			}

			if l.Current(id) != nil {
				mu.Lock()
				jogando++
				mu.Unlock()
			}
		}()
	}

	wg.Wait()

	if jogando != 1 {
		t.Errorf("%d jogadores sentaram na mesma cadeira, esperado 1", jogando)
	}
}

func TestSalaComSenhaSóAbreComASenhaCerta(t *testing.T) {
	l := New()

	r := l.Create("ana", "Ana", "duelo", "xeque")

	if _, err := l.Join(r.ID(), "bia", "Bia", "outra"); !errors.Is(err, ErrWrongPassword) {
		t.Fatalf("senha errada devia ser recusada, veio %v", err)
	}
	if _, err := l.Join(r.ID(), "bia", "Bia", ""); !errors.Is(err, ErrWrongPassword) {
		t.Fatalf("sem senha devia ser recusada, veio %v", err)
	}

	entrou, err := l.Join(r.ID(), "bia", "Bia", "xeque")
	if err != nil {
		t.Fatalf("senha certa devia entrar: %v", err)
	}
	if entrou != r {
		t.Fatal("entrou numa sala diferente da criada")
	}
}

func TestSalaTrancadaApareceNaListaComOSinal(t *testing.T) {
	l := New()

	l.Create("ana", "Ana", "trancada", "xeque")
	l.Create("bia", "Bia", "aberta", "")

	for _, open := range l.List() {
		if quer := open.Title == "trancada"; open.Locked != quer {
			t.Errorf("sala %q: Locked=%v, queria %v", open.Title, open.Locked, quer)
		}
	}
}

func TestSenhaDaSalaIgnoraEspaçosNasPontas(t *testing.T) {
	l := New()

	r := l.Create("ana", "Ana", "duelo", "  xeque  ")

	if _, err := l.Join(r.ID(), "bia", "Bia", "xeque"); err != nil {
		t.Fatalf("a senha aparada devia entrar: %v", err)
	}
}

func TestQuemJáEstáNaSalaTrancadaVoltaSemSenha(t *testing.T) {
	l := New()

	r := l.Create("ana", "Ana", "duelo", "xeque")

	// A reconexão da anfitriã não passa pela tela da senha: ela já está dentro.
	if volta, err := l.Join(r.ID(), "ana", "Ana", ""); err != nil || volta != r {
		t.Fatalf("a anfitriã devia voltar para a própria sala: %v", err)
	}
}
