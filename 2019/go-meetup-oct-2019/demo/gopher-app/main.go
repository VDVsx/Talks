package main

import (
	"fmt"
	"io"
	"io/ioutil"
	"math/rand"
	"net/http"
	"os"
	"strings"
	"sync"
	"html/template"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var maxEnergy = 50
var tiredState = maxEnergy / 2
var exhaustedState = int(float64(maxEnergy) * 0.1)
var gEnergy = maxEnergy

var mutex = &sync.Mutex{}

var exhaustedImg []string
var happyImg []string
var tiredImg []string

func initImagesLists() {
	exhaustedImg = initImageList("exhausted/")
	happyImg = initImageList("happy/")
	tiredImg = initImageList("tired/")
}

func initImageList(path string) []string {
	var ret []string
	files, err := ioutil.ReadDir("./gophers/" + path)
	if err != nil {
		panic(err.Error())
	}
	for _, f := range files {
		if strings.HasSuffix(f.Name(), ".png") {
			ret = append(ret, path+f.Name())
		}
	}
	return ret
}

func gopherEnergy() int {
	mutex.Lock()
	g := gEnergy
	mutex.Unlock()
	return g
}

func bumpGopherEnergy() int {
	mutex.Lock()
	if gEnergy < maxEnergy {
		gEnergy++
	}
	ret := gEnergy
	mutex.Unlock()
	return ret
}

func decreaseGopherEnergy() int {
	mutex.Lock()
	if gEnergy > 0 {
		gEnergy--
	}
	ret := gEnergy
	mutex.Unlock()
	return ret
}

func root(w http.ResponseWriter, r *http.Request) {
	html := `<html><body>
		<h1>Gopher Service</h1>
		<form action="upload" method="post" enctype="multipart/form-data">
			<input type="file" name="image">
			<button type="submit">Upload a Gopher</button>
		</form>
		</body></html`
	fmt.Fprint(w, html)
}

func energy(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "<h1>Gopher Energy %d</h1>", gopherEnergy())
}

func uploadGopher(w http.ResponseWriter, r *http.Request) {
	file, header, err := r.FormFile("image")
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	defer file.Close()
	// super smart AI :P
	if !strings.Contains(header.Filename, "gopher") {
		http.Error(w, "Not a Gopher!", http.StatusBadRequest)
		log.WithFields(log.Fields{
			"host":      r.Host,
			"code":      http.StatusBadRequest,
			"file_name": header.Filename,
		}).Error("Not a Gopher")
		return
	}
	onDisk, err := os.Create("./gophers/uploaded/" + header.Filename)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer onDisk.Close()
	_, err = io.Copy(onDisk, file)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/list", http.StatusFound)
}

func listGophers(w http.ResponseWriter, r *http.Request) {
	html := `<html><body>
		{{range .}}
			<img style="width: 10%;" src="/gophers/uploaded/{{.Name}}">
		{{end}}
		</br>
		<a href="/">Upload a Gopher</a>
		</html></body>`
	tpl := template.Must(template.New("list-gophers").Parse(html))

	files, err := ioutil.ReadDir("./gophers/uploaded")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	tpl.Execute(w, files)
}

func helloGopher(w http.ResponseWriter, r *http.Request) {
	var imgPath string
	// gopher can't go below min energy level for health reasons :)
	ge := gopherEnergy()
	idx := rand.Intn(3)
	switch {
	case ge <= 0:
		m := "Gopher is exhausted, cannot greet at the moment"
		log.WithFields(log.Fields{
			"host":         r.Host,
			"code":         http.StatusServiceUnavailable,
			"gopherEnergy": ge,
		}).Error(m)
		http.Error(w, m, http.StatusServiceUnavailable)
		return
	case ge <= exhaustedState:
		imgPath = exhaustedImg[idx]
		log.WithFields(log.Fields{
			"host":         r.Host,
			"code":         http.StatusOK,
			"gopherEnergy": ge,
		}).Warn("Gopher is getting too tired!")
	case ge < tiredState:
		imgPath = tiredImg[idx]
	default:
		imgPath = happyImg[idx]
	}
	decreaseGopherEnergy()
	w.WriteHeader(http.StatusOK)
	html := `<html><body>
		<h1>Moi!</h1>
		</br>
		<img style="width: 20%;" src="/gophers/{{.}}">
		</html></body>`
	tpl := template.Must(template.New("hello-gophers").Parse(html))
	err := tpl.Execute(w, imgPath)
	if err != nil {
		log.WithFields(log.Fields{
			"host":  r.Host,
			"error": err.Error(),
		}).Info("Got an error")
	}
}

// instrumentCounter instruments the handler with a request counter
// grouped by method and status code
func instrumentCounter(endpoint string, hFunc http.HandlerFunc) http.HandlerFunc {
	return promhttp.InstrumentHandlerCounter(
		promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: fmt.Sprintf("%v_requests_total", endpoint),
				Help: "A counter for requests to the wrapped handler.",
			},
			[]string{"code", "method"},
		),
		hFunc,
	)
}

func instrumentDuration(endpoint string, hFunc http.HandlerFunc) http.HandlerFunc {
	return promhttp.InstrumentHandlerDuration(
		promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    fmt.Sprintf("%v_duration_seconds", endpoint),
				Help:    "A histogram of latencies for requests.",
				Buckets: []float64{.25, .5, 1, 2.5, 5, 10},
			},
			[]string{"handler", "method", "code"},
		).MustCurryWith(prometheus.Labels{"handler": endpoint}),
		hFunc,
	)
}

func buildInstrumentation(endpoint string, hFunc http.HandlerFunc) http.HandlerFunc {
	return instrumentDuration(endpoint, instrumentCounter(endpoint, hFunc))
}

func main() {
	rand.Seed(time.Now().Unix())
	initImagesLists()

	// start a ticker to bump gopher energy every 5 seconds
	go func() {
		c := time.Tick(5 * time.Second)
		for range c {
			bumpGopherEnergy()
		}
	}()

	mux := http.NewServeMux()
	mux.Handle("/", buildInstrumentation("gopher_root", root))
	mux.Handle("/hello", buildInstrumentation("gopher_hello", helloGopher))
	mux.Handle("/energy", buildInstrumentation("gopher_energy", energy))
	mux.Handle("/upload", buildInstrumentation("gopher_upload", uploadGopher))
	mux.Handle("/list", buildInstrumentation("gopher_list", listGophers))

	fs := http.FileServer(http.Dir("./gophers"))
	mux.Handle("/gophers/", http.StripPrefix("/gophers", fs)) // not instrumented on purpose

	mux.Handle("/metrics", promhttp.Handler())
	log.Fatal(http.ListenAndServe(":8000", mux))
}
