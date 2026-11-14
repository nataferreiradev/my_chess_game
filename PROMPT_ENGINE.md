# Prompt — o motor de xadrez que joga contra o humano

## Contexto

Este repo (`github.com/nataferreiradev/my_chess_game`) é um servidor de xadrez jogável por SSH:
o usuário roda `ssh xadrez.meudominio.com`, cai numa TUI e joga. O cliente é o `ssh` puro dele —
tudo roda no servidor.

As **regras** já estão prontas e validadas. O que falta é o **adversário de computador**: dada uma
posição, escolher um lance. É isso que este prompt manda construir.

Vocabulário, para não confundir com o `engine_test.go` que já existe na raiz: ali "engine" quer
dizer *motor de regras* (o que sabe que o cavalo anda em L). Aqui "motor" quer dizer *jogador de
computador* (o que sabe que dama vale mais que peão). São coisas diferentes e vivem em packages
diferentes.

**Leia `HANDOFF.md` e `README.md` antes de começar.** Eles têm a API e a arquitetura. O que segue
aqui é o escopo, as armadilhas e a ordem de trabalho.

## Antes de tudo: o toolchain da máquina está quebrado

`/usr/lib/golang/VERSION` diz `go1.25.11` mas pertence ao RPM 1.26.7, e sobraram arquivos de
transação interrompida. O conserto é `sudo dnf reinstall golang golang-bin golang-src` (peça ao
dono da máquina; você não tem sudo). Enquanto isso não acontece, use o toolchain íntegro já
baixado:

```sh
export GOROOT=/home/nata/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.26.4.linux-amd64
$GOROOT/bin/go test ./...
```

Se você validar com o workaround, **diga isso no relatório**. Não reporte verde sem avisar.

## O que já existe, e é seu ponto de partida

O package raiz é **puro**: só as regras, sem `net`, sem canal, sem sessão. A API pública:

```go
const BoardSize = 64
const InitialFEN = "rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1"

type Color byte              // Black=0, White=1
type Board []*GamePiece      // 64 casas, nil = vazia. Índice 0 = a8, índice 63 = h1.
type Outcome byte            // Ongoing, Checkmate, Stalemate, DrawFiftyMove, DrawThreefold, DrawInsufficientMaterial

func NewGame() *RunningGame
func NewGameFromFEN(fen string) (*RunningGame, error)

func (g *RunningGame) Board() Board                       // cópia profunda
func (g *RunningGame) PieceAt(idx int) (GamePiece, bool)
func (g *RunningGame) CurrentPlayer() Color
func (g *RunningGame) LegalMoves() []string               // notação algébrica, ordem indefinida
func (g *RunningGame) MakeMove(cmd string) (AppliedMove, error)
func (g *RunningGame) Outcome() Outcome
func (g *RunningGame) InCheck() bool
func (g *RunningGame) History() []AppliedMove
func (g *RunningGame) FEN() string

func SquareIndex(sq parser.Square) (int, error)           // e conversões irmãs
func ParseSquareName(name string) (int, error)
func SquareName(idx int) string
```

As regras estão validadas por **perft** (`engine_test.go`, `TestPerft`): posição inicial até a
profundidade 4, kiwipete até 3, final com en passant até 4, promoções até 3. Todos batem com os
números publicados. **Isso é o seu chão: se um perft mudar depois de uma alteração sua, você
quebrou as regras.**

### A estrutura do motor já está criada

```
internal/engine/
  engine.go       Engine, Options, Level, Result, Best — a fachada. Stub.
  search.go       negamax, quiescência, controle de tempo. Stub.
  eval.go         avaliação da posição parada. Stub.
  order.go        ordenação de lances. Stub.
  engine_test.go  os testes de contrato — a sua lista de tarefas.
```

Tudo compila e `go test ./...` passa hoje, porque os testes de contrato se marcam como pendentes
atrás de `const implementada = false` no topo de `engine_test.go`. **Troque para `true` e faça
todos passarem** — é esse o alvo. Não afrouxe teste para ficar verde; se um teste estiver errado,
diga por que e proponha o certo.

### Quanto custa hoje um nó (medido, não estimado)

| caminho | medida |
|---|---|
| `perft` interno (candidatos + `Clone` por nó) | 197.281 nós em ~120 ms → **~1,6 M nós/s** |
| pela API pública (`LegalMoves` + `FEN` + `MakeMove` por nó) | 8.902 nós em ~22 ms → **~400 k nós/s** |

A API pública é inutilizável numa busca: cada nó reparseia notação, remonta a partida da FEN e
realoca o tabuleiro. E mesmo o caminho interno clona o tabuleiro duas vezes por lance (uma no
filtro de legalidade, outra no perft). **Por isso a fase 1 existe.**

## A decisão de projeto que você precisa respeitar

1. **O package raiz continua puro.** Nada de busca, avaliação ou heurística nele.
2. **O motor mora em `internal/engine`** e conversa com as regras pela API pública.
3. **O caminho rápido de lance é exportado pelo package raiz** (fase 1). O motor não pode ter o
   seu próprio tabuleiro: duas implementações das regras divergem, e o dia em que divergirem o
   motor vai jogar um lance que as regras recusam — com o adversário humano olhando.
4. **Sem dependência nova.** O repo depende do `chess_notation_parser` e do Charm (só o servidor).
   O motor é stdlib pura.

Assinaturas sugeridas para o caminho rápido — ajuste se tiver argumento melhor, mas diga qual:

```go
// Move é um lance já resolvido: sem notação, sem ambiguidade, sem parse.
type Move struct{ /* from, to, promoção, roque, en passant */ }

// Undo carrega o que Apply destruiu, para Unapply desfazer sem clonar nada.
type Undo struct{ /* peça capturada, direitos de roque, en passant, relógio */ }

func (g *RunningGame) Moves() []Move          // os legais da vez, sem alocar por chamada se der
func (g *RunningGame) Apply(m Move) Undo      // aplica no lugar
func (g *RunningGame) Unapply(u Undo)         // volta exatamente ao estado anterior
func (g *RunningGame) Notation(m Move) string // só na raiz da busca, para devolver o lance
```

## Sete armadilhas

1. **As coordenadas são invertidas em relação ao parser.** `parser.Square{File, Rank}` tem
   `Rank 0` = linha 1, mas o índice 0 do tabuleiro é a8. A conversão já existe (`SquareIndex`).
   Confira sempre: a8→0, e4→36, h1→63.
2. **O espelho da tabela por casa é `idx ^ 56`, não `63 - idx`.** `63 - idx` espelha *também* as
   colunas, o que estraga qualquer tabela assimétrica (rei no roque curto, por exemplo).
   `idx ^ 56` troca só a linha, que é o que você quer para ler a tabela das brancas pelo lado das
   pretas.
3. **Peão branco anda para índices menores** (`-8`), preto para maiores (`+8`), porque a linha 8
   fica no começo do slice.
4. **A FEN não é a partida.** `Best` trabalha sobre uma cópia montada da FEN, e a FEN não carrega
   o histórico de posições. A cópia, portanto, **não enxerga repetição tripla** — o motor pode
   entrar num empate por repetição achando que está ganhando, ou deixar de forçá-lo quando está
   perdendo. Decida o que fazer (passar as chaves de posição junto, ou aceitar e documentar) e
   diga qual escolheu. Não deixe isso implícito.
5. **Material insuficiente muda o escore.** Capturar a última peça do adversário pode transformar
   uma posição ganha em empate imediato (`DrawInsufficientMaterial`). A avaliação tem de olhar
   `Outcome`, não só somar peças. O teste `TestPegaAPeçaPendurada` usa peões extras justamente
   para não cair nisso — repare por quê antes de mexer nele.
6. **`LegalMoves()` devolve a notação sem `+` e sem `#`.** Não dá para saber se um lance dá xeque
   olhando a string. Quem sabe é a posição depois do lance.
7. **`RunningGame` não é seguro para concorrência, e a sala é a dona dela.** O árbitro da sala
   roda numa goroutine só (ver `internal/room`). Quando o motor entrar lá (fase 6), o árbitro tem
   de tirar a FEN, mandar o motor pensar **em outra goroutine**, e receber o lance de volta como um
   comando qualquer — nunca chamar `Best` dentro do laço do árbitro, ou a sala inteira congela
   enquanto o computador pensa, e o adversário humano não consegue nem desistir.

## Fases

**Faça uma fase por vez e pare para eu revisar. Não emende fases.**

### Fase 0 — linha de base (sem escrever código de produção)
Rode `TestPerft`, meça e anote os números da sua máquina. Escreva um benchmark (`BenchmarkPerft`)
que sirva de referência para as fases seguintes. Entregue os números.

### Fase 1 — caminho rápido de lances no package raiz
`Move`, `Undo`, `Moves`, `Apply`, `Unapply`, `Notation`. Reescreva o `perft` de `engine_test.go`
em cima deles.

Critério: **os mesmos números de perft** (qualquer divergência é bug seu), pelo menos **5× mais
rápido** que a linha de base da fase 0, e **zero alocação por nó** (`-benchmem` → `0 allocs/op`
no laço interno). O filtro de legalidade que clona o tabuleiro inteiro por lance é o primeiro
lugar para olhar.

### Fase 2 — avaliação
`evaluate` em `eval.go`: material, tabela por casa (as seis, com o espelho da armadilha 2), par de
bispos, peão dobrado e isolado, e abrigo do rei. Nada de olhar o futuro: quem olha o futuro é a
busca. Misturar as duas é o jeito mais rápido de ficar com um motor que ninguém depura.

Critério: **teste de simetria** — a avaliação de uma posição espelhada tem de dar exatamente o
escore com o sinal trocado. É o teste que pega 90% dos erros de sinal e de espelho.

### Fase 3 — busca
Negamax com alfa-beta e aprofundamento iterativo, variante principal, escore de mate contado por
`ply` (para preferir o mate mais curto), teto de tempo e `context` respeitados. Preencha `run` e
`negamax`, e faça `Best` usá-los.

Critério: `TestAchaOMateEmUm` passa; acrescente um mate em 2 e faça passar. `TestRespeitaOTempo` e
`TestParaNoContextoCancelado` passam. `TestNãoMexeNaPartidaRecebida` passa.

### Fase 4 — quiescência e ordenação
`quiescence` e `orderMoves`: MVV-LVA nas capturas, killers por ply, history. Meça quantos nós a
busca gasta para chegar à profundidade 6 antes e depois.

Critério: `TestNãoDeixaAPrópriaPeçaDeGraça` e `TestNãoAfogaComVantagem` passam, e a contagem de nós
para a mesma profundidade cai (diga quanto).

### Fase 5 — níveis
`Level.Options`, o `Slack` aleatório do nível fácil, `Seed` para reprodutibilidade. Escreva o
`TestNívelDifícilGanhaDoFácil` que está marcado como TODO.

Critério: com `Seed` fixa, duas execuções dão a mesma partida. Com `Slack > 0`, partidas
diferentes — e nunca um lance que perde material grosso de graça.

### Fase 6 — plugar no servidor
Menu da TUI: "jogar contra o computador", com escolha de nível. Sala com um assento de bot. O
árbitro pede o lance ao motor **fora da sua goroutine** (armadilha 7) e o aplica como um comando
qualquer quando ele chega. A sala tem de continuar respondendo enquanto o computador pensa:
`desistir` e `menu` funcionam no meio da pensada.

Critério: um teste e2e por SSH (no estilo dos que já existem em `cmd/xadrez-ssh/e2e_test.go`) em
que o jogador cria uma partida contra o computador, joga `e4`, e o computador responde.

## Critérios de aceite, no fim

- `go test ./...` verde, `gofmt -l .` vazio, `go vet ./...` limpo.
- `implementada = true` e **todos** os testes de contrato passando.
- Os números de perft **idênticos** aos de hoje.
- O nível difícil responde em até 3 s por lance.
- Autojogo de 120 lances sem nenhum lance recusado pelas regras.

## Regras de trabalho

- **Comentário em português, explicando o porquê, não o quê.** Olhe qualquer arquivo do repo: os
  comentários dizem por que a decisão foi tomada e o que quebra se alguém desfizer. Siga isso.
  Nome de teste em português também, como o resto.
- **Teste junto com o código, não depois.** Cada fase entrega testes que falhariam sem ela.
- **Não mexa na TUI, no lobby nem na sala fora da fase 6.**
- **Não afrouxe teste para passar.** Se um teste estiver errado, argumente e proponha o certo.
- Se uma medição sua contradisser este documento, **acredite na medição** e me avise — os números
  daqui foram medidos em 2026-09-12 nesta máquina e podem ter envelhecido.
