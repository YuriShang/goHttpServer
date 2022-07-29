package main

import (
	"encoding/json"
	"errors"
	"flag"
	"io/ioutil"
	"log"
	"net/http"
	"strconv"
	"strings"
)

func middleware(next http.Handler) http.Handler {
	var (
		keys     []string
		oldValue string
		newValue string
		idx      int
		listTrig bool
	)

	flagVal := flag.String("r", `"keys", "oldValue", "newValue"`, "Правила перезаписи")
	flag.Parse()

	rules := strings.Split(*flagVal, `, `)
	for i, word := range rules {
		rules[i] = strings.Trim(word, `"()",`)
	}

	if len(rules) == 3 {
		keys = strings.Split(rules[0], ".")
		oldValue, newValue = rules[1], rules[2]
	} else {
		log.Fatal(`Правила перезаписи должны иметь формат "key, oldValue, newValue". 
		Если ключей несколько, необходимо записать их через точку: keys.foo.bar`)
	}

	if len(keys) > 1 {
		idx++
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		headerContentTtype := r.Header.Get("Content-Type")
		if headerContentTtype != "application/json" {
			errorResponse(w, "Content Type is not application/json", http.StatusUnsupportedMediaType)
		}

		var unmarshalErr *json.UnmarshalTypeError
		var raw map[string]interface{}

		decoder := json.NewDecoder(r.Body)
		err := decoder.Decode(&raw)

		// Проверяем JSON на валидность
		if err != nil {
			if errors.As(err, &unmarshalErr) {
				errorResponse(w, "Bad Request. Wrong Type provided for field "+unmarshalErr.Field, http.StatusBadRequest)
			} else {
				errorResponse(w, "Bad Request "+err.Error(), http.StatusBadRequest)
			}
		}

		// Парсим JSON
		for k, v := range raw {
			switch t := v.(type) {

			// Строка
			case string:
				if k == keys[0] && t == oldValue {
					raw[keys[0]] = newValue
					break
				}

			// Число
			case float64:
				oldValue64, _ := strconv.ParseFloat(oldValue, 64)
				newValue64, _ := strconv.ParseFloat(newValue, 64)
				if k == keys[0] && t == oldValue64 {
					raw[keys[0]] = newValue64
					break
				}

			// Вложенный словарь
			case map[string]interface{}:
				if idx < len(keys)-1 {
					idx++
				}
				parseMap(t, keys, oldValue, newValue, idx, listTrig)

			// Вложенный массив
			case []interface{}:
				parseList(t, keys, oldValue, newValue, idx, listTrig)
			}
		}

		result, err := json.Marshal(raw)
		if err != nil {
			log.Fatal(err)
		}
		r.Body = ioutil.NopCloser(strings.NewReader(string(result)))
		next.ServeHTTP(w, r)
	})
}

func parseMap(data map[string]interface{}, keys []string, old, new interface{}, idx int, listTrig bool) {
	// Функция парсит вложенный словарь

	for k, v := range data {
		// Инициализируем переключатель типов данных
		if data[keys[idx]] != nil {
			if idx < len(keys)-1 {
				idx++
			}
			switch val := v.(type) {
			case float64:
				old64, _ := strconv.ParseFloat(old.(string), 64)
				new64, _ := strconv.ParseFloat(new.(string), 64)
				if data[keys[idx]] == old64 && k == keys[len(keys)-1] {
					data[k] = new64
					return
				}
			case string:
				if data[keys[idx]] == old && k == keys[len(keys)-1] {
					data[k] = new
					return
				}
			case []interface{}:
				parseList(val, keys, old, new, idx, listTrig)
			case map[string]interface{}:
				parseMap(val, keys, old, new, idx, listTrig)
			}
		}
	}
}

func parseList(data []interface{}, keys []string, old, new interface{}, idx int, listTrig bool) {
	// Функция парсит вложенный массив
	for i, el := range data {
		// Инициализируем переключатель типов данных
		switch val := el.(type) {
		case float64:
			old64, _ := strconv.ParseFloat(old.(string), 64)
			new64, _ := strconv.ParseFloat(new.(string), 64)
			if val == old64 && listTrig == true {
				data[i] = new64
				return
			}
		case string:
			if val == old && listTrig == true {
				data[i] = new
				return
			}
		case map[string]interface{}:
			if val[keys[len(keys)-1]] != nil {
				listTrig = true
			}
			parseMap(val, keys, old, new, idx, listTrig)
		case []interface{}:
			parseList(val, keys, old, new, idx, listTrig)
		}
	}
}

func errorResponse(w http.ResponseWriter, message string, httpStatusCode int) {
	// Функция возвращает ошибку при получении некорректного JSON объкта
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(httpStatusCode)
	resp := make(map[string]string)
	resp["message"] = message
	jsonResp, _ := json.Marshal(resp)
	w.Write(jsonResp)
}

func echo(w http.ResponseWriter, r *http.Request) {
	//Функция возвращает обратно клиенту измененный JSON объект
	var request map[string]interface{}
	err := json.NewDecoder(r.Body).Decode(&request)
	if err != nil {
		log.Fatal(err)
	}
	requestJson, err := json.Marshal(request)
	if err != nil {
		log.Fatal(err)
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(requestJson)
}

func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("/", echo)
	http.ListenAndServe(":80", middleware(mux))
}
