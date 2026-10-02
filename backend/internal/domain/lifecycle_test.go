package domain

import (
	"slices"
	"testing"
)

func TestAllowedNext(t *testing.T) {
	tests := []struct {
		name    string
		history []EventType
		want    []EventType
	}{
		{"leer", nil, []EventType{Vorgemerkt, Beworben}},
		{"nach Vorgemerkt", []EventType{Vorgemerkt}, []EventType{Beworben, Zurueckgezogen}},
		{"nach Beworben", []EventType{Beworben}, []EventType{
			ScreeningGespraech, ChallengeErhalten, Interview, Kennenlerntag, AngebotErhalten,
			Absage, Zurueckgezogen, KeineRueckmeldung,
		}},
		{"nach ChallengeErhalten", []EventType{Beworben, ChallengeErhalten}, []EventType{
			ChallengeAbgegeben, Interview, Kennenlerntag, AngebotErhalten,
			Absage, Zurueckgezogen, KeineRueckmeldung,
		}},
		{"nach Interview", []EventType{Beworben, Interview}, []EventType{
			Interview, Kennenlerntag, AngebotErhalten, Absage, Zurueckgezogen, KeineRueckmeldung,
		}},
		{"nach Kennenlerntag", []EventType{Beworben, Kennenlerntag}, []EventType{
			AngebotErhalten, Absage, Zurueckgezogen, KeineRueckmeldung,
		}},
		{"nach AngebotErhalten", []EventType{Beworben, AngebotErhalten}, []EventType{
			AngebotAngenommen, AngebotAbgelehnt, Absage, Zurueckgezogen,
		}},
		{"nach Absage", []EventType{Beworben, Absage}, nil},
		{"nach Zurueckgezogen", []EventType{Vorgemerkt, Zurueckgezogen}, nil},
		{"nach AngebotAngenommen", []EventType{Beworben, AngebotErhalten, AngebotAngenommen}, nil},
		{"KeineRueckmeldung nach Beworben", []EventType{Beworben, KeineRueckmeldung}, []EventType{
			ScreeningGespraech, ChallengeErhalten, Interview, Kennenlerntag, AngebotErhalten,
			Absage, Zurueckgezogen,
		}},
		{"KeineRueckmeldung nach Interview", []EventType{Beworben, Interview, KeineRueckmeldung}, []EventType{
			Interview, Kennenlerntag, AngebotErhalten, Absage, Zurueckgezogen,
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := AllowedNext(tt.history)
			if !slices.Equal(got, tt.want) {
				t.Errorf("AllowedNext(%v)\n got  %v\n want %v", tt.history, got, tt.want)
			}
		})
	}
}
