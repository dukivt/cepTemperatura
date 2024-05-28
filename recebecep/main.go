package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"regexp"

	"github.com/go-chi/chi"
	"github.com/go-chi/chi/middleware"
	"go.opentelemetry.io/otel"
)

type Response struct {
	TempC float64 `json:"temp_c"`
	TempF float64 `json:"temp_f"`
	TempK float64 `json:"temp_k"`
	City  string  `json:"city"`
}

var paramCep struct {
	Cep string `json:"cep"`
}

func main() {
	startZipkin()

	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Logger)

	r.Post("/", ProcuraCepHandler)

	err := http.ListenAndServe(":8080", r)
	if err != nil {
		return
	}
}

func ProcuraCepHandler(w http.ResponseWriter, r *http.Request) {

	if r.URL.Path != "/" {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	body, err := io.ReadAll(r.Body)
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
		}
	}(r.Body)

	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("erro ao ler requisição"))
		return
	}

	err = json.Unmarshal(body, &paramCep)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("erro na requisição"))
		return
	}

	if paramCep.Cep == "" {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("o parâmetro cep é obrigatório"))
		return
	}

	validate := regexp.MustCompile(`^[0-9]{8}$`)
	if !validate.MatchString(paramCep.Cep) {
		w.WriteHeader(http.StatusUnprocessableEntity)
		w.Write([]byte("cep inválido"))
		return
	}

	temperature, err := TemperaturaCep(paramCep.Cep, r.Context())

	if err != nil {
		errorStr := err.Error()
		if errorStr == "não foi possível encontrar o CEP" {
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte(errorStr))
		} else {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("erro ao buscar o CEP: " + errorStr))
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")

	if temperature != nil {
		w.WriteHeader(http.StatusOK)
		err := json.NewEncoder(w).Encode(temperature)
		if err != nil {
			return
		}
	} else {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte("não foi possível encontrar a temperatura"))
	}
}

func TemperaturaCep(cep string, ctx context.Context) (*Response, error) {
	_, span := otel.Tracer("recebecep").Start(ctx, "chamando-temperatura-cep")
	defer span.End()

	req, err := http.NewRequestWithContext(ctx, "GET", "http://temperaturacep:8082/?cep="+cep, nil)
	if err != nil {
		return nil, err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode == http.StatusNotFound {
		return nil, errors.New("não foi possível encontrar o CEP")
	}

	res, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
		}
	}(resp.Body)

	var data Response
	err = json.Unmarshal(res, &data)
	if err != nil {
		return nil, err
	}

	return &data, nil
}
