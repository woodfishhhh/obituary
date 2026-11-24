package doctor

import (
	_ "embed"
	"errors"
	"html/template"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/woodfishhhh/obituary/internal/app"
	"github.com/woodfishhhh/obituary/internal/buildinfo"
	"github.com/woodfishhhh/obituary/internal/cli/cmdutil"
	initcmd "github.com/woodfishhhh/obituary/internal/cli/init"
)

// The report is one self-contained file: styles inline, no scripts, no
// requests to anywhere.
//
//go:embed report.html.tmpl
var reportSource string

var reportTemplate = template.Must(template.New("report").Parse(reportSource))

// openReport is replaced in tests; the real one starts the default browser.
var openReport = openFile

type reportView struct {
	Lang, Title, Headline, HeadlineClass string
	Meta                                 []reportMeta
	Categories                           []reportCategory
	CheckHead, StatusHead, DetailHead    string
	Privacy, Expires                     string
}

type reportMeta struct{ Label, Value string }

type reportCategory struct {
	Name, Class, Status, Count string
	Open                       bool
	Checks                     []reportCheck
}

type reportCheck struct {
	Name, Class, Status, Detail, Fix, Capabilities string
}

func statusClass(status string) string { return strings.ToLower(status) }

func (lang diagnostics) reportView(checks []DoctorCheck, now time.Time, home, executable string) reportView {
	view := reportView{
		Lang: string(lang.Language()), Title: lang.Text("cli.doctor.html.title", nil), Headline: lang.headline(checks),
		HeadlineClass: statusClass(categoryStatus(checks)),
		Meta: []reportMeta{
			{lang.Text("cli.doctor.html.generated", nil), now.Format("2006-01-02 15:04:05 -07:00")},
			{lang.Text("cli.doctor.html.version", nil), buildinfo.Version},
			{lang.Text("cli.doctor.html.system", nil), runtime.GOOS + "/" + runtime.GOARCH},
			{lang.Text("cli.doctor.html.executable", nil), executable},
			{lang.Text("cli.doctor.html.data_dir", nil), home},
		},
		CheckHead: lang.Text("cli.doctor.html.check", nil), StatusHead: lang.Text("cli.doctor.html.status", nil),
		DetailHead: lang.Text("cli.doctor.html.details", nil),
		Privacy:    lang.Text("cli.doctor.html.privacy", nil), Expires: lang.Text("cli.doctor.html.expires", nil),
	}
	for _, c := range groupChecks(checks) {
		group := reportCategory{
			Name: lang.categoryName(c.Key), Class: statusClass(c.Status), Status: lang.Label(c.Status),
			Count: lang.Count("cli.doctor.html.count", len(c.Checks)), Open: len(c.problems()) > 0,
		}
		for _, check := range c.Checks {
			row := reportCheck{Name: check.Name, Class: statusClass(check.Status), Status: lang.Label(check.Status), Detail: check.Detail}
			if fix := lang.fixFor(check); isProblem(check) && fix != "" {
				row.Fix = lang.Text("cli.doctor.summary.fix", map[string]any{"Fix": fix})
			}
			if check.Capabilities != nil {
				row.Capabilities = strings.TrimSpace(initcmd.CapabilityText(*check.Capabilities))
			}
			group.Checks = append(group.Checks, row)
		}
		view.Categories = append(view.Categories, group)
	}
	return view
}

func (lang diagnostics) renderReport(checks []DoctorCheck, now time.Time, home, executable string) ([]byte, error) {
	var out strings.Builder
	if err := reportTemplate.Execute(&out, lang.reportView(checks, now, home, executable)); err != nil {
		return nil, err
	}
	return []byte(out.String()), nil
}

// missingDataDirectory is returned instead of creating a data directory for a report.
type missingDataDirectory struct{ path string }

func (e missingDataDirectory) Error() string { return e.path }

// saveReport writes the report into the reports folder of the data directory
// and gives it its lifespan. A report that cannot be tracked would never
// expire, so it is removed again, along with the folder if this run made it.
func saveReport(report []byte, now time.Time) (path string, err error) {
	svc, err := cmdutil.Open()
	if err != nil {
		return "", err
	}
	if info, statErr := os.Stat(svc.Store.Home); statErr != nil || !info.IsDir() {
		return "", missingDataDirectory{svc.Store.Home} // the data directory is not ours to create
	}
	dir := app.ReportsDir(svc.Store.Home)
	_, statErr := os.Stat(dir)
	madeDir := errors.Is(statErr, fs.ErrNotExist)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	path, err = writeNewFile(dir, "doctor-"+now.Format("20060102-150405"), report)
	if err == nil {
		if err = app.ExpireReport(svc, path, now); err != nil {
			err = errors.Join(err, os.Remove(path))
			path = ""
		}
	}
	if err != nil && madeDir {
		_ = os.Remove(dir) // only succeeds while it is still empty
	}
	return path, err
}

// writeNewFile never replaces an earlier report, even within the same second.
func writeNewFile(dir, stem string, data []byte) (string, error) {
	for n := 1; n <= 20; n++ {
		name := stem + ".html"
		if n > 1 {
			name = stem + "-" + strconv.Itoa(n) + ".html"
		}
		path := filepath.Join(dir, name)
		f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
		if errors.Is(err, fs.ErrExist) {
			continue
		}
		if err != nil {
			return "", err
		}
		if _, err = f.Write(data); err != nil {
			err = errors.Join(err, f.Close(), os.Remove(path))
			return "", err
		}
		if err = f.Close(); err != nil {
			return "", errors.Join(err, os.Remove(path))
		}
		return path, nil
	}
	return "", &os.PathError{Op: "create", Path: filepath.Join(dir, stem+".html"), Err: fs.ErrExist}
}

// startDetached launches the default handler without waiting for it.
func startDetached(cmd *exec.Cmd) error {
	if err := cmd.Start(); err != nil {
		return err
	}
	return cmd.Process.Release()
}
