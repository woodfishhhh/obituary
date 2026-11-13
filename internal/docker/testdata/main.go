// Fake Docker accepts only the container commands used by Obituary.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"
)

type container struct {
	ID      string `json:"Id"`
	Created time.Time
	Name    string
	State   struct{ Running bool }
	Config  struct {
		Labels      map[string]string
		StopTimeout *int
	}
	HostConfig struct{ AutoRemove bool }
	Mounts     []struct{ Type, Source string }
	// PendingStop models work accepted by the engine surviving CLI termination.
	PendingStop time.Time `json:",omitempty"`
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	if path := os.Getenv("OBIT_FAKE_DOCKER_PARENTS"); path != "" {
		f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
		if err != nil {
			return err
		}
		fmt.Fprintln(f, os.Getppid())
		f.Close()
	}
	f, err := os.OpenFile(os.Getenv("OBIT_FAKE_DOCKER_LOG"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	err = json.NewEncoder(f).Encode(os.Args[1:])
	f.Close()
	if err != nil {
		return err
	}
	switch os.Getenv("OBIT_FAKE_DOCKER_MODE") {
	case "fail":
		return fmt.Errorf("daemon unavailable")
	case "sleep":
		time.Sleep(8 * time.Second)
		return nil
	}
	args, host := commandArgs(os.Args[1:])
	if len(args) == 0 {
		return fmt.Errorf("missing command")
	}
	if os.Getenv("OBIT_FAKE_DOCKER_AFTER_STOP_HANG") == args[0] {
		if _, err := os.Stat(os.Getenv("OBIT_FAKE_DOCKER_STATE") + ".stopped"); err == nil {
			time.Sleep(30 * time.Second)
		}
	}
	if args[0] == "context" {
		return contextCommand(args)
	}
	if args[0] == "info" {
		id := os.Getenv("OBIT_FAKE_DOCKER_DAEMON")
		if id == "" {
			id = "fake-daemon"
		}
		return json.NewEncoder(os.Stdout).Encode(map[string]string{"ID": id, "OSType": "windows"})
	}
	path := os.Getenv("OBIT_FAKE_DOCKER_STATE")
	if host != "npipe:////./pipe/fake-docker" && host != "" {
		path += ".other"
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var rows []container
	if err := json.Unmarshal(data, &rows); err != nil {
		return err
	}
	for i := range rows {
		if !rows[i].PendingStop.IsZero() && !time.Now().Before(rows[i].PendingStop) {
			rows[i].State.Running = false
			rows[i].PendingStop = time.Time{}
			if err := save(path, rows); err != nil {
				return err
			}
		}
	}
	if args[0] == "ps" {
		return list(path, args, rows)
	}
	return action(path, args, rows)
}

func commandArgs(args []string) ([]string, string) {
	host := os.Getenv("DOCKER_HOST")
	ctx := os.Getenv("DOCKER_CONTEXT")
	if len(args) > 1 && args[0] == "--host" {
		return args[2:], args[1]
	}
	if len(args) > 1 && args[0] == "--context" {
		ctx = args[1]
		args = args[2:]
		host = ""
	}
	if ctx != "" && ctx != "default" {
		host = contextHost(ctx)
	}
	return args, host
}

func contextHost(name string) string {
	if name == "default" {
		return "npipe:////./pipe/fake-docker"
	}
	if host := os.Getenv("OBIT_FAKE_DOCKER_CONTEXT_HOST"); host != "" {
		return host
	}
	return "tcp://" + name + ":2375"
}

func contextCommand(args []string) error {
	if args[1] == "show" {
		name := os.Getenv("DOCKER_CONTEXT")
		if name == "" {
			name = os.Getenv("OBIT_FAKE_DOCKER_CURRENT")
		}
		if name == "" {
			name = "default"
		}
		fmt.Println(name)
		return nil
	}
	name := args[len(args)-1]
	return json.NewEncoder(os.Stdout).Encode([]any{map[string]any{"Name": name, "Endpoints": map[string]any{"docker": map[string]string{"Host": contextHost(name)}}}})
}

func list(path string, args []string, rows []container) error {
	filter := ""
	for i, a := range args {
		if a == "--filter" {
			filter = strings.TrimSuffix(strings.TrimPrefix(args[i+1], "name=^/"), "$")
		}
	}
	if filter != "" && os.Getenv("OBIT_FAKE_DOCKER_COLLIDE_AFTER_CHECK") == filter {
		i := resolve(rows, filter)
		if i >= 0 {
			rows[i].Name = "/" + filter
			rows[i].ID = strings.Repeat("f", 64)
			if err := save(path, rows); err != nil {
				return err
			}
			return nil
		}
	}
	for _, row := range rows {
		if filter != "" && strings.TrimPrefix(row.Name, "/") != filter {
			continue
		}
		if err := json.NewEncoder(os.Stdout).Encode(row.ID); err != nil {
			return err
		}
	}
	return nil
}

func resolve(rows []container, ref string) int {
	for i, row := range rows {
		if row.ID == ref {
			return i
		}
	}
	for i, row := range rows {
		if strings.TrimPrefix(row.Name, "/") == strings.TrimPrefix(ref, "/") {
			return i
		}
	}
	match := -1
	for i, row := range rows {
		if strings.HasPrefix(row.ID, ref) {
			if match >= 0 {
				return -1
			}
			match = i
		}
	}
	return match
}

func action(path string, args []string, rows []container) error {
	if len(args) > 2 && args[0] == "container" && args[1] == "inspect" && args[2] == "--" {
		return inspect(path, args[3:], rows)
	}
	ref := args[len(args)-1]
	if args[0] == "rm" && os.Getenv("OBIT_FAKE_DOCKER_RM_REPLACE") == ref {
		if i := resolve(rows, ref); i >= 0 {
			rows = append(rows[:i], rows[i+1:]...)
		}
		if len(rows) > 0 {
			rows[0].Name = "/" + ref
		}
	}
	i := resolve(rows, ref)
	if i < 0 {
		return fmt.Errorf("No such container: %s", ref)
	}
	if len(args) != 3 || args[1] != "--" {
		return fmt.Errorf("unsafe command %v", args)
	}
	switch args[0] {
	case "stop":
		if err := stop(path, rows, i); err != nil {
			return err
		}
		rows[i].State.Running = false
		if err := os.WriteFile(path+".stopped", nil, 0600); err != nil {
			return err
		}
	case "start":
		rows[i].State.Running = true
	case "rm":
		if rows[i].State.Running {
			return fmt.Errorf("container is running")
		}
		rows = append(rows[:i], rows[i+1:]...)
	default:
		return fmt.Errorf("unsupported %v", args)
	}
	return save(path, rows)
}

func stop(path string, rows []container, i int) error {
	if delay := os.Getenv("OBIT_FAKE_DOCKER_STOP_DELAY"); delay != "" {
		d, err := time.ParseDuration(delay)
		if err != nil {
			return err
		}
		if os.Getenv("OBIT_FAKE_DOCKER_BACKEND_STOP") != "" {
			rows[i].PendingStop = time.Now().Add(d)
			if err := save(path, rows); err != nil {
				return err
			}
		}
		if p := os.Getenv("OBIT_FAKE_DOCKER_ENTERED"); p != "" {
			if err := os.WriteFile(p, []byte("stop"), 0600); err != nil {
				return err
			}
		}
		time.Sleep(d)
	}
	return nil
}

func save(path string, rows []container) error {
	data, err := json.Marshal(rows)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0600)
}

// inspect answers like docker: the containers it found, in argument order,
// then an error (exit 1) naming any reference it could not find.
func inspect(path string, refs []string, rows []container) error {
	var found []container
	var missing []string
	for _, ref := range refs {
		i := resolve(rows, ref)
		if i < 0 {
			missing = append(missing, ref)
			continue
		}
		row := rows[i]
		if os.Getenv("OBIT_FAKE_DOCKER_MODE") == "wrong-id" {
			row.ID = strings.Repeat("b", 64)
		}
		if os.Getenv("OBIT_FAKE_DOCKER_DISAPPEAR_AFTER_INSPECT") == ref {
			rows = append(rows[:i], rows[i+1:]...)
			if err := save(path, rows); err != nil {
				return err
			}
		}
		found = append(found, row)
	}
	if err := json.NewEncoder(os.Stdout).Encode(found); err != nil {
		return err
	}
	if len(missing) > 0 {
		return fmt.Errorf("No such container: %s", strings.Join(missing, ", "))
	}
	return nil
}
