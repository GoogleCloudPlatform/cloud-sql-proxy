// Package healthcheck tests and communicates the health of the Cloud SQL 
// Proxy Client.
package healthcheck

import "net/http"

var (
	live  bool
	ready bool
)

func initHealthCheck() {
	http.HandleFunc("/readiness", func(w http.ResponseWriter, r *http.Request) {
		if ready {
			w.WriteHeader(200)
			w.Write([]byte("ok\n"))
		} else {
			w.WriteHeader(500)
		}
	})

	http.HandleFunc("/liveness", func(w http.ResponseWriter, r *http.Request) {
		if live {
			w.WriteHeader(200)
			w.Write([]byte("ok\n"))
		} else {
			w.WriteHeader(500)
		}
	})

	go http.ListenAndServe(":8080", nil)
}
