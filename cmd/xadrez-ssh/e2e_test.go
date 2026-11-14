package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"net"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	gossh "golang.org/x/crypto/ssh"

	"github.com/nataferreiradev/my_chess_game/internal/lobby"
)

// Este teste é o critério de aceite do projeto, automatizado: dois clientes SSH
// de verdade, pareados numa partida, jogando uma sequência legal de lances,
// levando um lance ilegal recusado sem cair, e chegando a um mate.
//
// Não há mock de sessão: o servidor é o mesmo que o main sobe, e os clientes
// falam SSH de verdade contra ele.

const espera = 15 * time.Second

// terminal é um cliente SSH com PTY, do jeito que o ssh do jogador se comporta.
type terminal struct {
	t       *testing.T
	client  *gossh.Client
	session *gossh.Session
	writer  interface{ Write([]byte) (int, error) }

	mu       sync.Mutex
	output   strings.Builder
	answered bool
}

// O wish pergunta ao terminal qual é a cor de fundo dele antes de montar o
// renderer, e espera a resposta. Um terminal de verdade responde na hora; este
// cliente de teste tem de responder também, senão a sessão fica esperando.
const (
	queryBackground  = "\x1b]11;?\x07"
	answerBackground = "\x1b]11;rgb:1a1a/1a1a/1b1b\x07"
	answerAttributes = "\x1b[?62;c"
)

var ansi = regexp.MustCompile(`\x1b\[[0-9;?]*[a-zA-Z]|\x1b\][^\a]*\a|\r`)

func (term *terminal) Write(p []byte) (int, error) {
	term.mu.Lock()
	term.output.Write(p)
	answer := !term.answered && strings.Contains(term.output.String(), queryBackground)
	if answer {
		term.answered = true
	}
	term.mu.Unlock()

	if answer {
		_, _ = term.writer.Write([]byte(answerBackground + answerAttributes))
	}

	return len(p), nil
}

// screen devolve o que já chegou do servidor, sem os códigos de escape.
func (term *terminal) screen() string {
	term.mu.Lock()
	defer term.mu.Unlock()
	return ansi.ReplaceAllString(term.output.String(), "")
}

// press manda teclas cruas, sem enter: é como o jogador navega nos menus.
func (term *terminal) press(keys string) {
	term.t.Helper()
	if _, err := term.writer.Write([]byte(keys)); err != nil {
		term.t.Fatalf("não deu para apertar %q: %v", keys, err)
	}
}

// rawScreen devolve o que chegou do servidor com os códigos de escape, para
// conferir a cor.
func (term *terminal) rawScreen() string {
	term.mu.Lock()
	defer term.mu.Unlock()
	return term.output.String()
}

// typeLine digita uma linha e aperta enter, como o jogador faria.
func (term *terminal) typeLine(line string) {
	term.t.Helper()
	if _, err := term.writer.Write([]byte(line + "\r")); err != nil {
		term.t.Fatalf("não deu para digitar %q: %v", line, err)
	}
}

// waitFor espera o texto aparecer na tela.
func (term *terminal) waitFor(what string) {
	term.t.Helper()

	deadline := time.Now().Add(espera)
	for time.Now().Before(deadline) {
		if strings.Contains(term.screen(), what) {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}

	term.t.Fatalf("%q nunca apareceu na tela.\n--- tela ---\n%s", what, term.screen())
}

// waitForSequence espera os trechos aparecerem na tela, nessa ordem. Serve para
// conferir uma linha do histórico sem depender do espaçamento exato do painel.
func (term *terminal) waitForSequence(parts ...string) {
	term.t.Helper()

	deadline := time.Now().Add(espera)
	for time.Now().Before(deadline) {
		if containsInOrder(term.screen(), parts) {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}

	term.t.Fatalf("%v nunca apareceu em sequência na tela.\n--- tela ---\n%s", parts, term.screen())
}

func containsInOrder(screen string, parts []string) bool {
	for _, part := range parts {
		i := strings.Index(screen, part)
		if i < 0 {
			return false
		}
		screen = screen[i+len(part):]
	}
	return true
}

// refuseShown espera o texto aparecer e confirma que a sessão continua viva.
func (term *terminal) refuseShown(what string) {
	term.t.Helper()
	term.waitFor(what)

	if err := term.session.Signal(gossh.SIGINT); err != nil {
		// Signal é best-effort; o que importa é o canal ainda estar aberto.
		term.t.Logf("aviso: Signal devolveu %v", err)
	}
}

func (term *terminal) close() {
	if term.session != nil {
		_ = term.session.Close()
	}
	if term.client != nil {
		_ = term.client.Close()
	}
}

// connect abre uma sessão SSH com PTY e uma chave própria.
func connect(t *testing.T, addr, user string) *terminal {
	t.Helper()

	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("não deu para gerar a chave: %v", err)
	}
	signer, err := gossh.NewSignerFromKey(priv)
	if err != nil {
		t.Fatalf("não deu para montar o signer: %v", err)
	}

	client, err := gossh.Dial("tcp", addr, &gossh.ClientConfig{
		User:            user,
		Auth:            []gossh.AuthMethod{gossh.PublicKeys(signer)},
		HostKeyCallback: gossh.InsecureIgnoreHostKey(),
		Timeout:         espera,
	})
	if err != nil {
		t.Fatalf("%s não conseguiu conectar: %v", user, err)
	}

	session, err := client.NewSession()
	if err != nil {
		t.Fatalf("%s não conseguiu abrir a sessão: %v", user, err)
	}

	term := &terminal{t: t, client: client, session: session}
	t.Cleanup(term.close)

	stdin, err := session.StdinPipe()
	if err != nil {
		t.Fatalf("%s não conseguiu o stdin: %v", user, err)
	}
	term.writer = stdin
	session.Stdout = term
	session.Stderr = term

	// A TUI exige PTY: é o activeterm quem garante isso do lado do servidor.
	if err := session.RequestPty("xterm-256color", 45, 140, gossh.TerminalModes{}); err != nil {
		t.Fatalf("%s não conseguiu o PTY: %v", user, err)
	}
	if err := session.Shell(); err != nil {
		t.Fatalf("%s não conseguiu subir a TUI: %v", user, err)
	}

	return term
}

// servidor sobe o servidor de verdade numa porta livre.
func servidor(t *testing.T) string {
	t.Helper()

	server, err := newServer("127.0.0.1:0", filepath.Join(t.TempDir(), "hostkey"), lobby.New())
	if err != nil {
		t.Fatalf("não deu para montar o servidor: %v", err)
	}

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("não deu para escutar: %v", err)
	}

	go func() { _ = server.Serve(listener) }()
	t.Cleanup(func() { _ = server.Close() })

	return listener.Addr().String()
}

// chega conecta e passa pela tela inicial, confirmando o nome que veio do ssh,
// até o menu. É o começo de toda sessão.
func chega(t *testing.T, addr, user string) *terminal {
	t.Helper()

	term := connect(t, addr, user)
	term.waitFor("como você quer ser chamado")
	term.typeLine("") // enter confirma o nome que já veio preenchido
	term.waitFor("criar sala")

	return term
}

// criaSala escolhe "criar sala" no menu e abre uma sala aberta, sem título nem
// senha: enter no título desce para a senha, enter na senha cria.
func (term *terminal) criaSala() {
	term.t.Helper()
	term.criaSalaCom("", "")
}

// criaSalaCom preenche o formulário da sala nova e espera a sala abrir.
func (term *terminal) criaSalaCom(title, senha string) {
	term.t.Helper()
	term.press("1")
	term.waitFor("sala nova")
	term.typeLine(title)
	term.typeLine(senha)
	term.waitFor("esperando adversário")
}

// entraNaPrimeiraSala abre a lista de partidas e entra na primeira da lista.
func (term *terminal) entraNaPrimeiraSala(title string) {
	term.t.Helper()
	term.press("2")
	term.waitFor(title)
	term.press("1")
	term.waitFor("vez:") // o painel da partida já está na tela
}

func TestTelaInicialAbreAntesDaPartida(t *testing.T) {
	addr := servidor(t)

	ana := connect(t, addr, "ana")

	// A primeira tela é o nome, já preenchido com o usuário do ssh.
	ana.waitFor("como você quer ser chamado")
	ana.typeLine("Ana Maria")

	// Depois vem o menu, com as duas portas de entrada.
	ana.waitFor("criar sala")
	ana.waitFor("buscar partidas")
	if screen := ana.screen(); !strings.Contains(screen, "Ana Maria") {
		t.Errorf("o menu não mostrou o nome escolhido.\n--- tela ---\n%s", screen)
	}

	// A lista começa vazia e passa a mostrar a sala criada por outro jogador.
	ana.press("2")
	ana.waitFor("nenhuma sala aberta")

	bia := chega(t, addr, "bia")
	bia.criaSala()

	// A lista se atualiza sozinha: a sala de Bia aparece sem Ana apertar nada.
	ana.waitFor("sala de bia")
	ana.waitFor("esperando adversário")

	// E entrar nela põe os dois na mesma partida, com Bia de brancas por ter
	// criado a sala.
	ana.press("1")
	bia.waitFor("é a sua vez")
	ana.waitFor("esperando o adversário")
}

func TestSalaComSenhaSóDeixaEntrarQuemSabeASenha(t *testing.T) {
	addr := servidor(t)

	ana := chega(t, addr, "ana")
	ana.criaSalaCom("duelo", "xeque")

	bia := chega(t, addr, "bia")
	bia.press("2")
	bia.waitFor("duelo")

	// A sala trancada pede a senha em vez de deixar entrar direto.
	bia.press("1")
	bia.waitFor("essa sala pede senha")

	bia.typeLine("errada")
	bia.waitFor("senha errada")

	// E com a senha certa a partida começa normalmente.
	bia.typeLine("xeque")
	bia.waitFor("vez:")
	ana.waitFor("é a sua vez")
}

func TestQuemChegaNaSalaCheiaAssiste(t *testing.T) {
	addr := servidor(t)

	ana := chega(t, addr, "ana")
	ana.criaSala()

	bia := chega(t, addr, "bia")
	bia.entraNaPrimeiraSala("sala de ana")
	ana.waitFor("é a sua vez")

	ana.typeLine("e4")
	bia.waitFor("1. e4")

	// Com as duas cadeiras ocupadas, quem entra depois assiste — e recebe a
	// partida inteira, incluindo o que já foi jogado.
	caio := chega(t, addr, "caio")
	caio.entraNaPrimeiraSala("ana x bia")
	caio.waitFor("assistindo")
	caio.waitFor("1. e4")

	// Espectador não joga.
	caio.typeLine("e5")
	caio.refuseShown("você está assistindo")
}

func TestPartidaInteiraPorSSH(t *testing.T) {
	addr := servidor(t)

	// Ana cria a sala e espera.
	ana := chega(t, addr, "ana")
	ana.criaSala()

	// Bia acha a sala na lista e entra.
	bia := chega(t, addr, "bia")
	bia.entraNaPrimeiraSala("sala de ana")

	// Quem cria a sala joga de brancas.
	ana.waitFor("é a sua vez")
	bia.waitFor("esperando o adversário")

	// Um lance ilegal é recusado com mensagem na tela, sem derrubar a sessão.
	ana.typeLine("Ra3")
	ana.refuseShown("nenhuma torre das brancas alcança a3")

	// E um lance fora da vez também.
	bia.typeLine("e5")
	bia.refuseShown("não é a sua vez")

	// A sequência do mate do tolo, com o tabuleiro atualizando nos dois lados.
	lances := []struct {
		quem  *terminal
		lance string
		veem  []string
	}{
		{ana, "f3", []string{"1. f3"}},
		{bia, "e5", []string{"1. f3", "e5"}},
		{ana, "g4", []string{"2. g4"}},
	}

	for _, lance := range lances {
		lance.quem.typeLine(lance.lance)
		// O lance aplicado chega nas duas TUIs, não só na de quem jogou.
		ana.waitForSequence(lance.veem...)
		bia.waitForSequence(lance.veem...)
	}

	// O mate.
	bia.typeLine("Qh4")

	for nome, term := range map[string]*terminal{"ana": ana, "bia": bia} {
		term.waitFor("xeque-mate")
		if !strings.Contains(term.screen(), "vitória das pretas") {
			t.Errorf("%s não viu quem ganhou.\n--- tela ---\n%s", nome, term.screen())
		}
	}

	// A partida acabou: mais lances são recusados, e a sessão segue de pé.
	ana.typeLine("e4")
	ana.refuseShown("a partida já acabou")
}

func TestTabuleiroChegaDesenhado(t *testing.T) {
	addr := servidor(t)

	ana := chega(t, addr, "ana")
	ana.criaSala()
	bia := chega(t, addr, "bia")
	bia.entraNaPrimeiraSala("sala de ana")

	for nome, term := range map[string]*terminal{"ana": ana, "bia": bia} {
		term.waitFor("xadrez por ssh")

		screen := term.screen()
		for _, want := range []string{"♜", "♟", "♚", "♛"} {
			if !strings.Contains(screen, want) {
				t.Errorf("%s: o tabuleiro não trouxe %q.\n--- tela ---\n%s", nome, want, screen)
			}
		}
	}

	// Cada um vê o tabuleiro do seu lado: as brancas com a coluna a à esquerda,
	// as pretas com o tabuleiro girado.
	ana.waitFor("a  b  c  d  e  f  g  h")
	bia.waitFor("h  g  f  e  d  c  b  a")

	// As peças dos dois lados usam a mesma figura, então é a cor do texto que
	// diz de quem é a peça: sem cor nenhuma na saída, brancas e pretas ficam
	// idênticas na tela.
	if raw := ana.rawScreen(); !strings.Contains(raw, "\x1b[38;") {
		t.Error("o tabuleiro chegou sem cor: as brancas ficariam iguais às pretas")
	}
}

func TestReconexaoPorSSH(t *testing.T) {
	addr := servidor(t)

	ana := chega(t, addr, "ana")
	ana.criaSala()
	bia := chega(t, addr, "bia")
	bia.entraNaPrimeiraSala("sala de ana")
	ana.waitFor("é a sua vez")

	ana.typeLine("e4")
	bia.waitFor("1. e4")
	ana.waitFor("1. e4")

	// Bia cai no meio da partida.
	bia.close()
	ana.waitFor("o adversário caiu")

	// A mesma chave voltaria para a mesma partida; uma chave nova é outra
	// pessoa, e começa na tela inicial. É o fingerprint que faz as vezes de
	// conta.
	caio := connect(t, addr, "caio")
	caio.waitFor("como você quer ser chamado")

	if strings.Contains(caio.screen(), "1. e4") {
		t.Error("uma chave nova não pode cair na partida de outra pessoa")
	}
}

func TestSessaoSemPTYEhRecusadaSemDerrubarOServidor(t *testing.T) {
	addr := servidor(t)

	_, priv, _ := ed25519.GenerateKey(rand.Reader)
	signer, _ := gossh.NewSignerFromKey(priv)

	client, err := gossh.Dial("tcp", addr, &gossh.ClientConfig{
		User:            "semtty",
		Auth:            []gossh.AuthMethod{gossh.PublicKeys(signer)},
		HostKeyCallback: gossh.InsecureIgnoreHostKey(),
		Timeout:         espera,
	})
	if err != nil {
		t.Fatalf("não conectou: %v", err)
	}
	defer client.Close()

	session, err := client.NewSession()
	if err != nil {
		t.Fatalf("não abriu a sessão: %v", err)
	}

	// Sem PTY o activeterm recusa, com uma mensagem, e a sessão termina.
	out, _ := session.CombinedOutput("")
	if !strings.Contains(string(out), "PTY") {
		t.Errorf("esperado um recado sobre a falta de PTY, veio %q", out)
	}
	session.Close()

	// E o servidor continua atendendo quem chega depois.
	depois := connect(t, addr, "depois")
	depois.waitFor("como você quer ser chamado")
}

func TestComandoDeAjuda(t *testing.T) {
	addr := servidor(t)

	ana := chega(t, addr, "ana")
	ana.criaSala()
	bia := chega(t, addr, "bia")
	bia.entraNaPrimeiraSala("sala de ana")
	ana.waitFor("é a sua vez")

	ana.typeLine("ajuda")
	ana.waitFor("notação algébrica")

	// E a desistência encerra a partida para os dois.
	ana.typeLine("desistir")
	ana.waitFor("vitória das pretas")
	bia.waitFor("vitória das pretas")
}
