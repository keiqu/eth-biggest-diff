package main

import (
	"flag"
	"fmt"
	"log"
	"math/big"
	"net/http"
	"time"

	"golang.org/x/time/rate"
)

type accounts map[string]*big.Int

func (a accounts) update(transactions []Transaction) {
	for _, t := range transactions {
		if t.To == "" {
			continue
		}

		// subtract from the sending account
		if _, ok := a[t.From]; !ok {
			a[t.From] = new(big.Int)
		}
		a[t.From].Sub(a[t.From], t.Value)

		// add to the receiving account
		if _, ok := a[t.To]; !ok {
			a[t.To] = new(big.Int)
		}
		a[t.To].Add(a[t.To], t.Value)
	}
}

func (a accounts) getMax() (addr string, value string) {
	max := &big.Int{}
	for k, v := range a {
		if r := v.CmpAbs(max); r == 1 {
			addr = k
			max = v
		}
	}

	return addr, max.String()
}

func main() {
	token := flag.String("token", "", "Etherscan API token.")
	flag.Parse()

	var limit rate.Limit
	if *token == "" {
		limit = rate.Every(5 * time.Second)
	} else {
		limit = rate.Limit(5)
	}

	api := &etherscanAPI{
		client: &http.Client{
			Timeout: time.Second * 10,
		},
		rateLimiter: rate.NewLimiter(limit, 1),
		token:       *token,
	}

	addr, value := getMaxBalanceChange(api, 100)
	fmt.Printf("address: %s\nvalue: %s\n", addr, value)
}

func getMaxBalanceChange(api *etherscanAPI, numBlocks int) (addr string, value string) {
	lastBlock, err := api.getLastBlockNum()
	if err != nil {
		log.Fatal(err)
	}

	queue := make(chan uint64, numBlocks)
	for i := 0; i < numBlocks; i++ {
		queue <- lastBlock - uint64(i)
	}

	out := make(chan []Transaction)
	for i := 0; i < 10; i++ {
		go fetcher(api, queue, out)
	}

	b := accounts{}
	for i := 0; i < numBlocks; i++ {
		b.update(<-out)
	}

	return b.getMax()
}

func fetcher(api *etherscanAPI, queue chan uint64, out chan<- []Transaction) {
	for {
		select {
		case num := <-queue:
			info, err := api.getBlockInfo(num)
			if err != nil {
				queue <- num
				if err != nil {
					log.Println(err)
				}
				break
			}

			log.Printf("Got info about block %d\n", num)
			out <- info.Transactions
		default:
			// if queue is empty
			return
		}
	}
}
