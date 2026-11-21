package app

import (
	"errors"
	"fmt"
	"time"

	"github.com/woodfishhhh/obituary/internal/core"
)

// updateLeftoverLifespan is how long the previous programs of a manual install
// stay beside the new ones. They are then buried, and can be resurrected until
// the graveyard purges them.
const updateLeftoverLifespan = 24 * time.Hour

// retireUpdateFiles gives the folder an update leaves in a manual install's
// directory its lifespan, as obit mark would.
func retireUpdateFiles(paths []string) error {
	svc, err := Open()
	if err != nil {
		return err
	}
	return expireUpdateFiles(svc, paths)
}

func expireUpdateFiles(svc *core.Service, paths []string) error {
	results, err := svc.Add(paths, core.AddOptions{ExpiresAt: time.Now().Add(updateLeftoverLifespan), Source: "update", Note: "Obituary update: previous programs"})
	for _, r := range results {
		if r.Error != "" {
			err = errors.Join(err, fmt.Errorf("%s: %s", r.Ref, r.Error))
		}
	}
	return err
}
