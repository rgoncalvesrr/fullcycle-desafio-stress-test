package pkg

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"sync"
	"time"
)

type Input struct {
	Url      string
	Requests int
	Workers  int
}

func ExecuteTests(input *Input) {
	fmt.Printf("Rodando Stress Test for %s\n", input.Url)
	fmt.Printf("Requisições %d\n", input.Requests)
	fmt.Printf("Threads %d\n", input.Workers)
	fmt.Println("\nProcessando...")

	statusList, elapsedTime, err := do(input.Url, input.Requests, input.Workers)
	if err != nil {
		log.Fatal(err)
	}

	rpt := reportGenerate(statusList, elapsedTime)

	json.NewEncoder(os.Stdout).Encode(rpt)
}

func doRequest(url string) (http.Response, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		log.Printf("falha ao criar a requisição: %v", err)
		return http.Response{}, err
	}

	res, err := http.DefaultClient.Do(req)

	if err != nil {
		log.Printf("falha na requisição: %v", err)
		return *res, err
	}
	defer res.Body.Close()

	ctx_err := ctx.Err()
	if ctx_err != nil {
		select {
		case <-ctx.Done():
			err := ctx.Err()
			log.Printf("tempo limite atingido: %v", err)
			return *res, err
		}
	}

	return *res, nil
}

func do(url string, requests, concurrency int) ([]int, time.Duration, error) {
	var codeList []int
	var mu sync.Mutex
	var wg sync.WaitGroup
	semaphore := make(chan struct{}, concurrency)

	start := time.Now()

	for i := 0; i < requests; i++ {
		wg.Add(1)

		go func(i int) {
			defer wg.Done()
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			res, err := doRequest(url)
			if err != nil {
				log.Printf("Requisições #%d falharam: %v", i, err)
				return
			}

			mu.Lock()
			codeList = append(codeList, res.StatusCode)
			mu.Unlock()
		}(i)
	}
	wg.Wait()
	duration := time.Since(start)

	return codeList, duration, nil
}

type Report struct {
	TimeSpent          string         `json:"tempo_decorrido"`
	RequestsMade       int            `json:"executas"`
	SuccessfulRequests int            `json:"bem_sucedidas"`
	FailedRequests     map[string]int `json:"falhas"`
}

func reportGenerate(status_codes []int, duration time.Duration) *Report {
	r := len(status_codes)
	sucesso := 0
	falhas := map[string]int{}

	code_3xx := 0
	code_4xx := 0
	code_5xx := 0

	for _, code := range status_codes {
		if code == 200 {
			sucesso++
		} else {
			if code >= 300 && code <= 399 {
				code_3xx++
			}
			if code >= 400 && code <= 499 {
				code_4xx++
			}
			if code >= 500 && code <= 599 {
				code_5xx++
			}
		}
	}

	falhas["3xx"] = code_3xx
	falhas["4xx"] = code_4xx
	falhas["5xx"] = code_5xx

	report := &Report{
		TimeSpent:          duration.Round(time.Second).String(),
		RequestsMade:       r,
		SuccessfulRequests: sucesso,
		FailedRequests:     falhas,
	}

	return report
}
