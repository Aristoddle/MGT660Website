/*
Copyright 2014 Google Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

     http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// outyet is a web server that announces whether or not a particular Go version
// has been tagged.
package main

import (
	"expvar"
	"flag"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"sync"
	"time"
	"io/ioutil"
	"encoding/json"
)

// Command-line flags.
var (
	httpAddr   = flag.String("http", ":8080", "Listen address")
	pollPeriod = flag.Duration("poll", 5*time.Second, "Poll period")
	version    = flag.String("version", "1.4", "Go version")
)

const baseChangeURL = "https://go.googlesource.com/go/+/"

type LocationStruct struct {
	name	string
	region	string
	country	string
	lat		float64
	lon		float64
	tz_id	string
	localtime_epoch	int
	localtime	string
}

type ConditionStruct struct {
	text string
	icon string
	code int
}

type CurrentStruct struct {
	last_updated_epoch int
	last_updated string
	temp_c float64
	temp_f float64
	is_day int
	condition ConditionStruct
	wind_mph float64
	wind_kph float64
	wind_degree int
	wind_dir string
	pressure_mb float64
	pressure_in float64
	precip_mm float64
	precip_in float64
	humidity int
	cloud int
	feelslike_c int
	feelslike_f int
	vis_km float64
	vis_miles float64
}

type WeatherResponse struct {
	location LocationStruct
	current CurrentStruct

}

type CurrWeatherStruct struct {
	city string
	region string
	temp float64
	weathertext string
	weathericon string
}

func login(w http.ResponseWriter, r *http.Request) {
    fmt.Println("method:", r.Method) //get request method
    if r.Method == "GET" {
        t, _ := template.ParseFiles("login.gtpl")
        t.Execute(w, nil)
	
		} else {
        r.ParseForm()
		// logic part of log in
		var zipstr = r.Form["username"][0]
		var apistr = fmt.Sprintf("http://api.apixu.com/v1/current.json?key=95e45cd6aa314aafa2d212449172809&q=%s", zipstr)
		fmt.Println("\n\n zip code:\n\n", zipstr)
		
		client := &http.Client{}
		request, _ := http.NewRequest("GET", apistr, nil)
		request.Header.Set("x-api-key", "515007af1d9e005b2f02dd450238ceef")
		fmt.Println("request:", request)
		response, _ := client.Do(request)
		
		fmt.Println(response)
		body, _ := ioutil.ReadAll(response.Body)
		
		var res WeatherResponse
		fmt.Println("\n\n\n RES, PRE \n\n", res)
		if err := json.Unmarshal([]byte(string(body)), &res); err != nil {
			fmt.Println(err)
		}
		fmt.Println("\n\n body:\n\n", string(body))
		fmt.Println("\n\n body BYTES:\n\n", []byte(string(body)))
		fmt.Println("\n\n RES, POST:\n\n", res)

		var general map[string]interface{}
		if err := json.Unmarshal(body, &general); err != nil {
			panic(err)
		}
		fmt.Println(general)

		location := general["location"].(map[string]interface{})
		current := general["current"].(map[string]interface{})

		fmt.Println("set up current and loc...")
		cityname := location["name"].(string)
		regionname := location["region"].(string)
		temp := current["temp_f"].(float64)


		fmt.Println("\ncity, region, temp\t", cityname, regionname, temp)

		cond := current["condition"].(map[string]interface{})

		weathertext := cond["text"].(string)
		weathericon := cond["icon"].(string)

		fmt.Println("\n\n Pulled text and icon...\t", weathertext, weathericon)

		wthrstruct := CurrWeatherStruct { 
			cityname,
			regionname, 
			temp,
			weathertext,
			weathericon,
		}

		fmt.Println("\n\nwthrstruct: \t\t", wthrstruct)
		
		err := weatherResult.Execute(w, wthrstruct)
		if err != nil {
			log.Print(err)
		}
    }
}


func main() {
	flag.Parse()
	changeURL := fmt.Sprintf("%sgo%s", baseChangeURL, *version)
	http.Handle("/", NewServer(*version, changeURL, *pollPeriod))
	http.HandleFunc("/login", login)
	log.Fatal(http.ListenAndServe(*httpAddr, nil))
}

// Exported variables for monitoring the server.
// These are exported via HTTP as a JSON object at /debug/vars.
var (
	hitCount       = expvar.NewInt("hitCount")
	pollCount      = expvar.NewInt("pollCount")
	pollError      = expvar.NewString("pollError")
	pollErrorCount = expvar.NewInt("pollErrorCount")
)

// Server implements the outyet server.
// It serves the user interface (it's an http.Handler)
// and polls the remote repository for changes.
type Server struct {
	version string
	url     string
	period  time.Duration

	mu  sync.RWMutex // protects the yes variable
	yes bool
}

// NewServer returns an initialized outyet server.
func NewServer(version, url string, period time.Duration) *Server {
	s := &Server{version: version, url: url, period: period}
	go s.poll()
	return s
}

// poll polls the change URL for the specified period until the tag exists.
// Then it sets the Server's yes field true and exits.
func (s *Server) poll() {
	for !isTagged(s.url) {
		pollSleep(s.period)
	}
	s.mu.Lock()
	s.yes = true
	s.mu.Unlock()
	pollDone()
}

// Hooks that may be overridden for integration tests.
var (
	pollSleep = time.Sleep
	pollDone  = func() {}
)

// isTagged makes an HTTP HEAD request to the given URL and reports whether it
// returned a 200 OK response.
func isTagged(url string) bool {
	pollCount.Add(1)
	r, err := http.Head(url)
	if err != nil {
		log.Print(err)
		pollError.Set(err.Error())
		pollErrorCount.Add(1)
		return false
	}
	return r.StatusCode == http.StatusOK
}

// ServeHTTP implements the HTTP user interface.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	hitCount.Add(1)
	s.mu.RLock()
	data := struct {
		URL     string
		Version string
		Yes     bool
	}{
		s.url,
		s.version,
		s.yes,
	}
	s.mu.RUnlock()
	err := tmpl.Execute(w, data)
	if err != nil {
		log.Print(err)
	}
}

// tmpl is the HTML template that drives the user interface.
var tmpl = template.Must(template.New("tmpl").Parse(`
	<!DOCTYPE html>
	<html>
		<head>
		<title>Basic Weather App</title>
		</head>
		<body>
			<br><br><br>
			<form action="/login" method="post">
				Current Zip code:<input type="text" name="username">
				<input type="submit" value="Find Weather">
			</form>
		</body>
	</html>
`))

var weatherResult = template.Must(template.New("tmpl").Parse(`
	<!DOCTYPE html>
	<html>
		<head>
		<title>Basic Weather App</title>
		</head>
		<body>
		<br><br>
		<h1> The weather in {{.city}} , {{.region}} is: </h1>
		<h2> {{.weathertext}}, at {{.temp}} degrees Fahrenheit </h2>
		</body>
	</html>
`))
