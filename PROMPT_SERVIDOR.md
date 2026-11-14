# Prompt — terminar o servidor de xadrez por SSH

## Contexto

Estou construindo um servidor de xadrez jogável por SSH, no estilo do terminal.shop: o usuário
roda `ssh xadrez.meudominio.com` e cai direto numa TUI, sem instalar nada.

**O cliente é o `ssh` puro do usuário — ele não roda código meu.** Tudo (regras, TUI, salas)
roda no servidor. A notação que o jogador digita trafega como texto até o servidor, que parseia
e valida.

Duas bibliotecas minhas, em Go:

1. `github.com/nataferreiradev/my_chess_game` — **este repo. É o servidor.** Processa os lances,
   transmite aos clientes, e gerencia sessões e salas.
2. `github.com/nataferreiradev/chess_notation_parser` — helper de parse de notação algébrica,
   em `../chess_notation_parser`. Usado **apenas** pelo servidor.

**Leia `HANDOFF.md` na raiz deste repo antes de começar.** Ele tem o levantamento completo da API
atual, os bugs confirmados e a ordem sugerida de implementação. O que segue aqui é o escopo e as
regras de trabalho.

## Estado atual

A fundação já existe e os testes passam (`running_game_test.go`, 8 testes):

```go
const BoardSize = 64
type Color byte              // Black=0, White=1; Color.String() → "brancas"/"pretas"
type GamePiece struct { Piece parser.Piece; Color Color }
type Board []*GamePiece      // 64 casas, nil = vazia. Índice 0 = a8, índice 63 = h1.

func NewBoard() Board
func NewGame() *RunningGame
func (g *RunningGame) Board() Board                      // cópia profunda, de propósito
func (g *RunningGame) PieceAt(idx int) (GamePiece, bool)
func (g *RunningGame) CurrentPlayer() Color
func (g *RunningGame) MakeMove(cmd string) error         // ver Fase 1: vai mudar de assinatura
```

`MakeMove` hoje parseia, **descarta o resultado**, alterna a cor e devolve nil. Nunca escreve no
tabuleiro. O motor de xadrez está inteiro por fazer. O `TODO(fase-futura)` marca o ponto de entrada.

## Antes de tudo: destravar o ambiente

O toolchain Go da máquina está inconsistente — `/usr/lib/golang/VERSION` diz `go1.25.11` mas
pertence ao RPM 1.26.7, e sobraram arquivos de transação interrompida. Nada compila:
`asm: ABI selector only permitted when compiling runtime`.

Me peça para rodar `sudo dnf reinstall golang golang-bin golang-src` (você não tem sudo).
Enquanto isso não acontece, use o toolchain íntegro já baixado:

```sh
export GOROOT=/home/nata/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.26.4.linux-amd64
$GOROOT/bin/go test ./...
```

Se você validar com o workaround, **diga isso explicitamente** no relatório. Não reporte verde
em cima do workaround sem avisar.

## Três armadilhas que vão te custar tempo

Estão detalhadas no `HANDOFF.md`, mas repito porque são fáceis de pisar:

1. **As convenções de coordenada são invertidas.** `parser.Square{File, Rank}` tem `Rank 0` =
   linha 1, mas o índice 0 do tabuleiro = a8 (linha 8). A conversão é `idx = (7-Rank)*8 + File`.
   Confira: a8→0, e4→36, h1→63. **Faça essa função com teste antes de qualquer outra coisa.**
2. **Peão branco anda para índices menores** (`-8`), preto para maiores (`+8`), porque a linha 8
   fica no começo do slice.
3. **A desambiguação do parser está quebrada e falha em silêncio.** `Nbd2` vira `Nd2` com
   `From={-1,-1}`, sem erro; `From.Rank` nunca é preenchido. Conserte isso no
   `chess_notation_parser` **antes** de implementar a resolução da casa de origem — senão o motor
   vai rejeitar lances legais como "ambíguos" e a causa vai parecer estar no lugar errado.

## Fases

**Faça uma fase por vez e pare para eu revisar antes de seguir.** Não emende fases.

### Fase 1 — motor de xadrez
Todo o `HANDOFF.md` seções 3.A, 3.B, 3.C: conversão de coordenadas, alcance por peça, bloqueio de
caminho, resolução da casa de origem, validação de turno, captura, roque, en passant, promoção,
xeque, mate, afogamento, e os empates (50 lances, repetição tripla, material insuficiente).

Duas decisões de design a tomar **nesta fase**, porque mudar depois custa caro:
- `MakeMove` passa a devolver `(AppliedMove, error)` — o árbitro precisa saber *o quê* mudou para
  transmitir aos dois clientes, não só que deu certo. Ver `HANDOFF.md` 5.1.
- Implemente FEN import/export. Sem ele, testar roque ou mate exige jogar 30 lances até a posição.

Antes de codar, me mostre a assinatura final de `AppliedMove` e do FEN para eu aprovar.

### Fase 2 — corrigir o `chess_notation_parser`
Repo separado, em `../chess_notation_parser`. Roque (`O-O`, `O-O-O`), promoção (`e8=Q`),
`From.Rank`, desambiguação de peça maior, e rejeitar lixo (`Ne4e5` hoje passa sem erro).
Os testes existentes (`parser_test.go`) devem continuar passando.

Se a Fase 1 precisar disso antes (roque e promoção precisam), inverta a ordem e me avise.

### Fase 3 — lobby, salas e árbitro
Packages separados no mesmo módulo (`internal/lobby`, `internal/room` ou equivalente).

- **O package `my_chess_game` continua puro:** sem `net`, sem canais, sem `bubbletea`.
  `RunningGame` não pode saber o que é sessão SSH.
- Uma goroutine **árbitro** por partida é a única dona do `*RunningGame`. As sessões falam com ela
  por canais. Nunca compartilhe o `*RunningGame` entre goroutines diretamente — `Board()` já
  devolve cópia profunda justamente para isso.
- Identidade sem login: o fingerprint da chave pública SSH (`session.PublicKey()`) é a conta.
- Fila de pareamento: quem conecta entra numa fila e o servidor pareia dois.
- Trate desconexão no meio da partida, e reconexão (reenviando o histórico de lances).

### Fase 4 — servidor SSH e TUI
Stack Charm, **sem framework além dele**:
- `charmbracelet/wish` para o servidor SSH, exigindo PTY (`activeterm`), logando conexões
- `charmbracelet/bubbletea` para a TUI, uma instância por sessão
- `charmbracelet/lipgloss` para o estilo do tabuleiro
- `charmbracelet/bubbles` para input de texto e listas

Binário em `cmd/` deste repo (sugestão: `cmd/xadrez-ssh`), porta 23234.

Renderize com `bubbletea/middleware.MakeRenderer(session)` — o perfil de cor tem que ser o do
terminal do cliente, não o do servidor.

Tabuleiro com casas alternadas, coordenadas nas bordas, peças em Unicode (`♜♞♝♛♚♟`). Campo de
texto onde o jogador digita o lance em notação algébrica (`Nf3`, `exd5`, `Bxe4+`). Erros do parser
e do motor aparecem como mensagem na tela, **sem derrubar a sessão**. Cada lance aplicado é
propagado para as duas TUIs.

## Restrições

- Go idiomático. Sem framework além do Charm.
- **NÃO** reescreva a lógica de xadrez para usar `notnil/chess` ou outra lib de terceiros. O ponto
  do projeto é usar as minhas duas libs.
- Erros são retornados, nunca `panic`. Uma sessão que quebra não pode derrubar o servidor
  (`recover` por sessão).
- Testes para toda lógica nova de jogo. A camada de TUI pode ficar sem teste.
- Comentários e mensagens ao usuário **em português**, como já está no código.
- Não altere `Board()` para cópia rasa. É cópia profunda de propósito (ver `HANDOFF.md` 1).

## Critério de aceite

Por fase: `go build ./...` e `go test ./...` passam, com testes novos cobrindo a lógica da fase.

No fim: eu consigo abrir dois terminais, rodar `ssh -p 23234 localhost` em cada um, ser pareado
numa partida, jogar uma sequência legal de lances vendo o tabuleiro atualizar nos dois, levar um
lance ilegal recusado com mensagem na tela sem cair, e chegar a um mate detectado corretamente.
