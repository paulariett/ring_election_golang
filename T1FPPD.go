package main

import (
	"fmt"
	"math/rand"
	"sync"
	"time"
)

type mensagem struct {
	tipo  int    // 1=eleição, 2=falha, 3=recupera, 4=coordenador, 9=fim
	corpo [n]int // ids envolvidos
}

const n = 4

var (
	chans = []chan mensagem{ // vetor de canias para formar o anel de eleicao - chan[0], chan[1] and chan[2] ...
		make(chan mensagem),
		make(chan mensagem),
		make(chan mensagem),
		make(chan mensagem),
	}
	controle = make(chan int)
	wg       sync.WaitGroup // wg is used to wait for the program to finish
)

func Controlador(in chan int) {
	defer wg.Done()

	var temp mensagem
	r := rand.New(rand.NewSource(time.Now().UnixNano()))

	processosVivos := make([]bool, n)
	for i := range processosVivos {
		processosVivos[i] = true
	}

	const originalLeaderID = 0
	liderAtual := originalLeaderID
	numEventos := 12

	fmt.Println("--- CONTROLE: INICIANDO SIMULAÇÃO ---")

	for i := 0; i < numEventos; i++ {
		time.Sleep(1000 * time.Millisecond)
		fmt.Printf("\n--- Evento #%d ---\n", i+1)

		// drena todas as notificações pendentes de novo líder
		for {
			select {
			case novoLider := <-in:
				liderAtual = novoLider
				fmt.Printf("Controle: agora o líder é %d\n", liderAtual)
			default:
				goto eventos
			}
		}

	eventos:
		acao := r.Intn(2)

		if acao == 0 { // falha
			var alvos []int
			for id, vivo := range processosVivos {
				if vivo {
					alvos = append(alvos, id)
				}
			}
			if len(alvos) <= 1 {
				fmt.Println("Controle: poucos vivos, não falha.")
				continue
			}
			vitimaID := alvos[r.Intn(len(alvos))]
			indice := (vitimaID - 1 + n) % n
			fmt.Printf("Controle: FALHA no processo %d\n", vitimaID)

			temp.tipo = 2
			chans[indice] <- temp
			fmt.Println("Controle: aguardando confirmação...", <-in)
			processosVivos[vitimaID] = false

			// se o líder caiu, dispara eleição
			if vitimaID == liderAtual {
				fmt.Println("Controle: líder caiu, iniciando eleição...")
				iniciador := (vitimaID + 1) % n
				// escolhe o primeiro vivo adiante
				for k := 0; k < n && !processosVivos[iniciador]; k++ {
					iniciador = (iniciador + 1) % n
				}
				fmt.Printf("Controle: eleição iniciada pelo processo %d\n", iniciador)
				temp.tipo = 1
				temp.corpo = [n]int{iniciador, iniciador}
				chans[iniciador] <- temp
			}

		} else { // recuperação
			var mortos []int
			for id, vivo := range processosVivos {
				if !vivo {
					mortos = append(mortos, id)
				}
			}
			if len(mortos) == 0 {
				fmt.Println("Controle: ninguém para recuperar.")
				continue
			}
			alvoID := mortos[r.Intn(len(mortos))]
			indice := (alvoID - 1 + n) % n
			fmt.Printf("Controle: RECUPERA processo %d\n", alvoID)

			temp.tipo = 3
			chans[indice] <- temp
			fmt.Println("Controle: aguardando confirmação...", <-in)
			processosVivos[alvoID] = true

			// Se o líder original voltou, NÃO duplica coordenador aqui;
			// o próprio processo 0 já anuncia no ElectionStage (case 3).
			if alvoID == originalLeaderID {
				fmt.Printf("Controle: LÍDER ORIGINAL (P%d) voltou, reassumindo!\n", originalLeaderID)
				// liderAtual será atualizado quando o anúncio circular
			}
		}
	}

	time.Sleep(1000 * time.Millisecond)
	fmt.Println("\n CONTROLE: FIM DA SIMULAÇÃO ")
	temp.tipo = 9
	for i := 0; i < n; i++ {
		chans[(i-1+n)%n] <- temp
	}
}

func ElectionStage(id int, in chan mensagem, out chan mensagem, leader int) {
	defer wg.Done()

	bFailed := false
	actualLeader := leader

	for {
		temp := <-in
		switch temp.tipo {
		case 1: // eleição
			if bFailed {
				// mesmo falho, repassa para não quebrar o anel
				out <- temp
				continue
			}
			fmt.Printf("%d: recebeu eleição (iniciador=%d, maior=%d)\n", id, temp.corpo[0], temp.corpo[1])
			if id > temp.corpo[1] {
				temp.corpo[1] = id
				fmt.Printf("%d: atualizou maior ID para %d\n", id, temp.corpo[1])
			}
			if temp.corpo[0] == id {
				// eleição voltou ao iniciador
				actualLeader = temp.corpo[1]
				fmt.Printf("%d: novo líder eleito %d\n", id, actualLeader)
				controle <- actualLeader // avisa o controlador
				out <- mensagem{tipo: 4, corpo: [n]int{id, 0, actualLeader, 0}}
			} else {
				out <- temp
			}

		case 2: // falha
			bFailed = true
			fmt.Printf("%d: entrou em falha\n", id)
			controle <- id

		case 3: // recupera
			bFailed = false
			fmt.Printf("%d: recuperou\n", id)
			controle <- id
			if id == leader {
				// líder original reassume e anuncia
				actualLeader = id
				fmt.Printf("%d: reassumiu como líder original\n", id)
				controle <- id
				out <- mensagem{tipo: 4, corpo: [n]int{id, 0, id, 0}}
			}

		case 4: // coordenador
			// mesmo falho, repassa para não quebrar o anel
			if !bFailed {
				actualLeader = temp.corpo[2]
				fmt.Printf("%d: reconhece líder %d\n", id, actualLeader)
			}
			if temp.corpo[0] != id {
				out <- temp
			}

		case 9: // fim
			fmt.Printf("%d: encerrando\n", id)
			return
		}
	}
}

func main() {
	// adiciona no WaitGroup antes de iniciar
	wg.Add(n + 1)

	// cria processos em anel
	go ElectionStage(0, chans[n-1], chans[0], 0) // líder inicial
	for i := 0; i < n-1; i++ {
		go ElectionStage(i+1, chans[i], chans[i+1], 0)
	}

	fmt.Println("Anel criado")

	// controlador
	go Controlador(controle)

	wg.Wait()
}
