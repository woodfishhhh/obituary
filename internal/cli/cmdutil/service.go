package cmdutil

import (
	"github.com/woodfishhhh/obituary/internal/app"
	"github.com/woodfishhhh/obituary/internal/auto"
)

var Open = app.Open

func Engine() (*auto.Engine, error) {
	svc, err := Open()
	if err != nil {
		return nil, err
	}
	return app.Engine(svc), nil
}
