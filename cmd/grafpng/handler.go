/*
   Copyright 2018 Vastech SA (PTY) LTD

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
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/JaySeek/grafpng/grafana"
	"github.com/JaySeek/grafpng/report"
	"github.com/gorilla/mux"
)

// ServeReportHandler interface facilitates testsing the reportServing http handler
type ServeReportHandler struct {
	newGrafanaClient func(url string, apiToken string, variables url.Values) grafana.Client
	newReport        func(g grafana.Client, dashName string, time grafana.TimeRange, worker int) report.Report
}

// RegisterHandlers registers all http.Handler's with their associated routes to the router
// Two different serve report handlers are used to provide support for both Grafana v4 (and older) and v5 APIs
func RegisterHandlers(router *mux.Router, reportServerV4, reportServerV5 ServeReportHandler) {
	router.Handle("/api/report/{dashId}", reportServerV4)
	router.Handle("/api/v5/report/{dashId}", reportServerV5)
}

func (h ServeReportHandler) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	log.Print("Reporter called")
	gc := h.newGrafanaClient(*proto+*ip, apiToken(req), dashVariables(req))
	di := dashID(req)
	dt := dashTime(req)
	rep := h.newReport(gc, di, dt, *worker)

	file, err := rep.Generate()
	if err != nil {
		log.Println("Error generating report:", err)
		http.Error(w, err.Error(), 500)
		return
	}
	defer rep.Clean()
	defer file.Close()
	name := rep.Title() + dt.FromFormatted() + dt.ToFormatted()
	addFilenameHeader(w, name)

	_, err = io.Copy(w, file)
	if err != nil {
		log.Println("Error copying data to response:", err)
		http.Error(w, err.Error(), 500)
		return
	}
	log.Println("Report generated correctly")
}

func addFilenameHeader(w http.ResponseWriter, title string) {
	//sanitize title. Http headers should be ASCII
	filename := strconv.QuoteToASCII(title)
	filename = strings.TrimLeft(filename, "\"")
	filename = strings.TrimRight(filename, "\"")
	filename += ".png"
	log.Println("Extracted filename from dashboard title: ", filename)
	header := fmt.Sprintf("inline; filename=\"%s\"", filename)
	w.Header().Add("Content-Disposition", header)
}

func dashID(r *http.Request) string {
	vars := mux.Vars(r)
	d := vars["dashId"]
	log.Println("Called with dashboard:", d)
	return d
}

func dashTime(r *http.Request) grafana.TimeRange {
	params := r.URL.Query()
	t := grafana.NewTimeRange(params.Get("from"), params.Get("to"))
	log.Println("Called with time range:", t)
	return t
}

func apiToken(r *http.Request) string {
	apiToken := r.URL.Query().Get("apitoken")
	log.Println("Called with api Token:", apiToken)
	return apiToken
}

func dashVariables(r *http.Request) url.Values {
	output := url.Values{}
	for k, v := range r.URL.Query() {
		if strings.HasPrefix(k, "var-") {
			log.Println("Called with variable:", k, v)
			for _, singleV := range v {
				output.Add(k, singleV)
			}
		}
	}
	if len(output) == 0 {
		log.Println("Called without variable")
	}
	return output
}
