package hub

import "github.com/Baotluong/jotto-go/internal/game"

// View is one player's picture of a game. It never contains the opponent's
// secret until the game is over.
type View struct {
	Code           string        `json:"code"`
	Settings       game.Settings `json:"settings"`
	Phase          game.Phase    `json:"phase"`
	You            int           `json:"you"`
	Turn           int           `json:"turn"`
	LastChance     bool          `json:"lastChance"`
	Players        [2]PlayerView `json:"players"`
	YourSecret     string        `json:"yourSecret"`
	OpponentSecret string        `json:"opponentSecret,omitempty"`
	Guesses        []game.Guess  `json:"guesses"`
	Winner         int           `json:"winner"` // seat, 2 = draw, -1 = none
}

type PlayerView struct {
	Name      string `json:"name"`
	Joined    bool   `json:"joined"`
	Ready     bool   `json:"ready"` // secret chosen
	Connected bool   `json:"connected"`
}

func (r *room) viewFor(seat int) View {
	g := r.g
	v := View{
		Code:       g.Code,
		Settings:   g.Settings,
		Phase:      g.Phase,
		You:        seat,
		Turn:       g.Turn,
		LastChance: g.LastChance(),
		YourSecret: g.Players[seat].Secret,
		Guesses:    g.Guesses,
		Winner:     g.Winner,
	}
	if v.Guesses == nil {
		v.Guesses = []game.Guess{}
	}
	for i, p := range g.Players {
		if p == nil {
			continue
		}
		v.Players[i] = PlayerView{Name: p.Name, Joined: true, Ready: p.Secret != "", Connected: r.connected(i)}
	}
	if g.Phase == game.Finished {
		v.OpponentSecret = g.Players[1-seat].Secret
	}
	return v
}
