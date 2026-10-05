package httpapi

import (
	"net/http"
	"time"

	"golang.org/x/time/rate"
)

// Drossel der Agent-API: global statt je IP, weil Funnel und nginx die Client-Adresse verdecken.
// Angemeldete und fremde Anfragen haben getrennte Kontingente, damit eine Flut fremder Anfragen
// den legitimen Client (die Claude-Routine) nicht aussperrt.
// rate.Every ist eine Funktion, daher var statt const.
var (
	agentRate     = rate.Every(500 * time.Millisecond) // 2 Anfragen/s mit gültigem Token
	agentAnonRate = rate.Every(time.Second)            // 1 Anfrage/s ohne gültiges Token
)

const (
	agentBurst     = 20
	agentAnonBurst = 10
)

// agentLimits hält die Token-Buckets für Anfragen mit (auth) und ohne (anon) gültiges Token.
type agentLimits struct {
	auth, anon *rate.Limiter
}

// writeTooManyRequests antwortet mit 429; bei den Raten oben ist nach spätestens einer Sekunde wieder Platz.
func writeTooManyRequests(w http.ResponseWriter) {
	w.Header().Set("Retry-After", "1")
	writeProblem(w, problem{Type: problemBase + "too-many-requests", Title: "Zu viele Anfragen",
		Status: http.StatusTooManyRequests, Detail: "Bitte später erneut versuchen"})
}
