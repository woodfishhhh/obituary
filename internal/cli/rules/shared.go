package rules

import (
	"errors"
	"os"
	"path/filepath"

	"github.com/woodfishhhh/obituary/internal/pathutil"
	"github.com/woodfishhhh/obituary/internal/project"
	"github.com/woodfishhhh/obituary/internal/rules"
)

func CurrentProject() (project.Project, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return project.Project{}, err
	}
	p, ok, err := project.Find(cwd)
	if err != nil {
		return p, err
	}
	if !ok {
		return p, errors.New("not inside a Git work tree")
	}
	return p, nil
}

func RulesFile(args []string) (string, error) {
	if len(args) == 1 {
		return pathutil.Abs(args[0])
	}
	p, err := CurrentProject()
	if err != nil {
		return "", err
	}
	return filepath.Join(p.Root, rules.FileName), nil
}
