package main

import (
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"golang.org/x/net/html"
)

var (
	codewords = prometheus.NewGaugeVec(prometheus.GaugeOpts{Name: "codewords", Help: "codewords count"}, []string{"type", "channel"})
)

func init() {
	prometheus.MustRegister(codewords)
	http.Handle("/metrics", promhttp.Handler())
	go func() {
		log.Fatalln(http.ListenAndServe(":10000", nil))
	}()
}

func main() {
	req, err := http.NewRequest("GET", "http://routerip/comcast_network.php", nil)
	if err != nil {
		log.Println(err.Error())
		os.Exit(1)
	}

	req.Header.Add("Cookie", os.Args[1])
	tick := time.NewTicker(time.Minute).C
	for {
		<-tick
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			log.Println(err.Error())
			os.Exit(1)
		}

		z := html.NewTokenizer(resp.Body)

		stage := ""
		index := 1
		for z.Token().Data != "html" {
			tt := z.Next()
			if tt == html.StartTagToken {
				t := z.Token()
				if t.Data == "td" || t.Data == "th" {
					inner := z.Next()
					if inner == html.TextToken {
						text := (string)(z.Text())
						t := strings.TrimSpace(text)
						if t == "CM Error Codewords" {
							stage = t
						}
						if stage != "" {
							stage = t
							index = 1
						}
					}
					if inner == html.StartTagToken && stage != "" {
						t = z.Token()
						if t.Data == "div" {
							inner := z.Next()
							if inner == html.TextToken {
								text := (string)(z.Text())
								t := strings.TrimSpace(text)

								n, err := strconv.Atoi(t)
								if err != nil {
									log.Println(err.Error())
									break
								}
								switch stage {
								// "Unerrored Codewords" are unrelated to codewords addressed to the CM
								case "Correctable Codewords":
									codewords.WithLabelValues("correctable", strconv.Itoa(index)).Set(float64(n))
									index++
								case "Uncorrectable Codewords":
									codewords.WithLabelValues("uncorrectable", strconv.Itoa(index)).Set(float64(n))
									index++
								default:
									continue
								}
							}
						}

					}
				}
			}
		}
		resp.Body.Close()
	}
}
