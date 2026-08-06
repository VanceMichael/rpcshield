// Package lottery provides a Gateway to a lottery-playing service.
package lottery

import (
	"context"

	"thriftrw/lottery"
	"thriftrw/lottery/lotteryclient"
)

// Client represents the Lottery client interface.
type Client = lotteryclient.Interface

// Gateway plays the lottery, checking whether the chosen number is a winner.
type Gateway interface {
	Play(context.Context, int32) (bool, error)
}

type gateway struct {
	client Client
}

// New returns a Gateway for a lottery-playing service.
func New(client Client) Gateway {
	return &gateway{client: client}
}

// Play plays the lottery with the given lucky number, returning true if it won.
func (g *gateway) Play(ctx context.Context, lucky int32) (bool, error) {
	res, err := g.client.Play(ctx, &lottery.PlayRequest{Lucky: lucky})
	if err != nil {
		return false, err
	}
	return res.Won, nil
}
