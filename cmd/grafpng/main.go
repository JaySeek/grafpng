/*
   Copyright 2016 Vastech SA (PTY) LTD

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

package main

import (
	"flag"
	"log"
	"net/http"
	"os"

	"github.com/gorilla/mux"
	"github.com/JaySeek/grafpng/grafana"
	"github.com/JaySeek/grafpng/report"
)

var proto = flag.String("proto", "http://", "Grafana Protocol")
var ip = flag.String("ip", "localhost:3000", "Grafana Address")
var port = flag.String("port", ":8686", "Service Address")
var worker = flag.Int("worker", 2, "Service Workers")

func main() {
	flag.Parse()
	log.SetOutput(os.Stdout)
	log.Printf("serving at '%s' and using grafana at '%s'", *port, *ip)
	w := 1
	if *worker < 1 {
		worker = &w
	}

	router := mux.NewRouter()
	RegisterHandlers(
		router,
		ServeReportHandler{grafana.NewV4Client, report.NewReport},
		ServeReportHandler{grafana.NewV5Client, report.NewReport},
	)
	log.Fatal(http.ListenAndServe(*port, router))
}
