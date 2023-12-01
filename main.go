package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/eiannone/keyboard"
	"github.com/gosuri/uilive"
	"github.com/guptarohit/asciigraph"
	"io"
	"net/http"
	"strconv"
	"sync"
	"time"
)

var (
	ErrQuiteButton = errors.New("quite")
)

type LastData struct {
	BtcUsd CurrencyPair `json:"BTC_USD"`
	LtcUsd CurrencyPair `json:"LTC_USD"`
	EthUsd CurrencyPair `json:"ETH_USD"`
}

type CurrencyPair struct {
	PriceStr string `json:"avg"`
}

func main() {
	if err := menu(); err != nil {
		panic(err)
	}
}

func menu() error {
	const (
		end   rune = 113
		one   rune = 49
		two   rune = 50
		three rune = 51
	)

	data := make([]float64, 0, 120)

	writer := uilive.New()
	writer.Start()
	defer writer.Stop()

	if err := keyboard.Open(); err != nil {
		return fmt.Errorf("menu -> %w", err)
	}
	defer keyboard.Close()

	for {
		fmt.Fprintf(writer, "Main menu\n1. BTC_USD\n2. LTC_USD\n3. ETH_USD\n\nPress 1-3 to change symbol, press q to exit\n")
		char, _, err := keyboard.GetKey()
		if err != nil {
			return fmt.Errorf("menu -> %w", err)
		}

		switch char {
		case end:
			return nil
		case one:
			keyboard.Close()
			if err := buildPlot(writer, "BTC", data); err != nil {
				return fmt.Errorf("menu -> %w", err)
			}
			if err := keyboard.Open(); err != nil {
				return fmt.Errorf("menu -> %w", err)
			}
		case two:
			keyboard.Close()
			if err := buildPlot(writer, "LTC", data); err != nil {
				return fmt.Errorf("menu -> %w", err)
			}
			if err := keyboard.Open(); err != nil {
				return fmt.Errorf("menu -> %w", err)
			}
		case three:
			keyboard.Close()
			if err := buildPlot(writer, "ETH", data); err != nil {
				return fmt.Errorf("menu -> %w", err)
			}
			if err := keyboard.Open(); err != nil {
				return fmt.Errorf("menu -> %w", err)
			}
		default:
		}
	}
}

func buildPlot(writer *uilive.Writer, currency string, data []float64) error {
	keysEvents, err := keyboard.GetKeys(1)
	if err != nil {
		return fmt.Errorf("buildPlot -> %w", err)
	}
	defer keyboard.Close()

	ch := make(chan []float64)
	errCh := make(chan error, 1)
	defer close(errCh)
	ctx, closeCtx := context.WithCancel(context.Background())

	go func() {
		if err := exit(keysEvents); err != nil {
			errCh <- err
		}
	}()

	go func() {
		for {
			select {
			case <-ctx.Done():
				close(ch)
				return
			default:
			}

			if err := worker(currency, data, ch); err != nil {
				errCh <- err
			}

			time.Sleep(1 * time.Second)
		}
	}()

	go func() {
		for data = range ch {
			graph := asciigraph.Plot(data, asciigraph.SeriesColors(asciigraph.Red), asciigraph.Width(100), asciigraph.Height(10), asciigraph.Precision(3))

			fmt.Fprintf(writer, "%s_USD: %.3f\n", currency, data[len(data)-1])
			fmt.Fprintf(writer, "%s\n", graph)
			fmt.Fprintf(writer, "Текущее время: %v\n", time.Now().Format("15:04:05"))
			fmt.Fprintf(writer, "Текущая дата: %v\n", time.Now().Format("2006-01-02"))
		}
	}()

	err = <-errCh
	closeCtx()
	if errors.Is(err, ErrQuiteButton) {
		return nil
	}
	return fmt.Errorf("buildPlot -> %w", err)
}

func exit(keysEvents <-chan keyboard.KeyEvent) error {
	var err error
	wg := sync.WaitGroup{}

	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case key := <-keysEvents:
				if key.Err != nil {
					err = key.Err
					return
				}
				if key.Key == keyboard.KeyBackspace || key.Key == keyboard.KeyBackspace2 {
					err = ErrQuiteButton
					return
				}
			}
		}
	}()

	wg.Wait()
	return err
}

func worker(currency string, oldData []float64, ch chan []float64) error {
	newData, err := getNewData(currency, oldData)
	if err != nil {
		return fmt.Errorf("worker -> %w", err)
	}
	ch <- newData
	return nil
}

func getNewData(currency string, oldData []float64) ([]float64, error) {
	newData, err := getNewPrice(currency)
	if err != nil {
		return oldData, err
	}
	if len(oldData) == cap(oldData) {
		return append(append(oldData[:0], oldData[1:]...), newData), nil
	}
	return append(oldData, newData), nil
}

func getNewPrice(currency string) (float64, error) {
	rate, err := getExchangeRates()
	if err != nil {
		return 0, err
	}

	var data LastData
	err = json.Unmarshal(rate, &data)
	if err != nil {
		return 0, err
	}

	return getOnePrice(currency, &data)
}

func getExchangeRates() ([]byte, error) {
	client := &http.Client{}

	req, err := http.NewRequest("POST", "https://api.exmo.com/v1.1/ticker", nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	return io.ReadAll(resp.Body)
}

func getOnePrice(currency string, data *LastData) (float64, error) {
	var Price float64
	var err error

	switch currency {
	case "BTC":
		Price, err = strconv.ParseFloat(data.BtcUsd.PriceStr, 64)
	case "LTC":
		Price, err = strconv.ParseFloat(data.LtcUsd.PriceStr, 64)
	case "ETH":
		Price, err = strconv.ParseFloat(data.EthUsd.PriceStr, 64)
	default:
		err = fmt.Errorf("wrong currency: have %s\n avialable currencies: BTC, LTC, ETH\n", currency)
	}

	if err != nil {
		return 0, err
	}

	return Price, nil
}
