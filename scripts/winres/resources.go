package main

import (
	"bytes"
	"debug/pe"
	_ "embed"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/tc-hib/winres"
	"github.com/tc-hib/winres/version"
)

// Wails v2 internal/frontend/desktop/windows/winc/app.go loads AppIconID 3.
const appIconID = winres.ID(3)

// Wails registers its window class with LoadIcon(instance, IDI_APPLICATION)
// and sets only the small window icon from appIconID, so the taskbar's large
// icon comes from this ID. Its group points at the same images.
const classIconID = winres.ID(32512)

//go:embed obituary.manifest
var manifest []byte

var iconSizes = []int{16, 20, 24, 32, 40, 48, 64, 128, 256}

type application struct {
	name, description string
	arches            []winres.Arch
	subsystem         uint16
}

var applications = []application{
	{"obit", "命令行", []winres.Arch{winres.ArchAMD64, winres.ArchARM64}, pe.IMAGE_SUBSYSTEM_WINDOWS_CUI},
	{"obitd", "后台与托盘", []winres.Arch{winres.ArchAMD64, winres.ArchARM64}, pe.IMAGE_SUBSYSTEM_WINDOWS_GUI},
	{"obit-gui", "小窗口", []winres.Arch{winres.ArchAMD64}, pe.IMAGE_SUBSYSTEM_WINDOWS_GUI},
}

func generate(root, release, only string) error {
	if only != "" && only != "obit" && only != "obitd" && only != "obit-gui" {
		return fmt.Errorf("unknown application %q", only)
	}
	icon, err := brandIcon(root)
	if err != nil {
		return err
	}
	for _, app := range applications {
		if only != "" && only != app.name {
			continue
		}
		rs, err := resources(root, app, release, icon)
		if err != nil {
			return err
		}
		for _, arch := range app.arches {
			var object bytes.Buffer
			if err := rs.WriteObject(&object, arch); err != nil {
				return err
			}
			path := filepath.Join(root, "cmd", app.name, "resource_windows_"+string(arch)+".syso")
			if err := os.WriteFile(path, object.Bytes(), 0o644); err != nil {
				return err
			}
		}
	}
	var ico bytes.Buffer
	if err := icon.SaveICO(&ico); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(root, "internal", "tray", "obituary.ico"), ico.Bytes(), 0o644)
}

func brandIcon(root string) (*winres.Icon, error) {
	data, err := os.ReadFile(filepath.Join(root, "docs", "assets", "brand", "obituary-icon-128.png"))
	if err != nil {
		return nil, err
	}
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	// The repository's largest square source is 128px; Lanczos resizing in
	// tc-hib/winres supplies the requested 256px layer as well as the tray sizes.
	return winres.NewIconFromResizedImage(squareAroundMark(img), iconSizes)
}

// squareAroundMark crops the brand image to a square around the tombstone.
// The source draws it in the middle half of a 128px canvas with wide grass,
// which left the mark only a few pixels tall in the taskbar and tray.
func squareAroundMark(img image.Image) image.Image {
	b := img.Bounds()
	alpha := func(x, y int) uint8 { return color.NRGBAModel.Convert(img.At(x, y)).(color.NRGBA).A }
	x0, y0, x1, y1 := b.Max.X, b.Max.Y, b.Min.X-1, b.Min.Y-1
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			if alpha(x, y) > 40 {
				x0, y0, x1, y1 = min(x0, x), min(y0, y), max(x1, x), max(y1, y)
			}
		}
	}
	if x1 < x0 {
		return img
	}
	// The stone is the solid part of the upper rows; the grass below spreads
	// wider and must not pull the centre sideways.
	var sum, n int
	for y := y0; y <= y0+(y1-y0)*6/10; y++ {
		for x := x0; x <= x1; x++ {
			if alpha(x, y) > 200 {
				sum += x
				n++
			}
		}
	}
	cx := (x0 + x1) / 2
	if n > 0 {
		cx = sum / n
	}
	pad := (y1 - y0 + 1) / 25
	side := y1 - y0 + 1 + 2*pad
	src := image.Rect(cx-side/2, y0-pad, cx-side/2+side, y0-pad+side)
	out := image.NewNRGBA(image.Rect(0, 0, side, side))
	draw.Draw(out, out.Bounds(), img, src.Min, draw.Src)
	return out
}

func resources(root string, app application, release string, icon *winres.Icon) (*winres.ResourceSet, error) {
	vi, err := versionInfo(root, app, release)
	if err != nil {
		return nil, err
	}
	rs := &winres.ResourceSet{}
	rs.SetVersionInfo(*vi)
	if err := rs.SetIcon(appIconID, icon); err != nil {
		return nil, err
	}
	if err := rs.Set(winres.RT_GROUP_ICON, classIconID, winres.LCIDNeutral, rs.Get(winres.RT_GROUP_ICON, appIconID, winres.LCIDNeutral)); err != nil {
		return nil, err
	}
	// A custom manifest preserves activeCodePage, which winres.AppManifest
	// does not expose; the resource encoding remains entirely in the library.
	if err := rs.Set(winres.RT_MANIFEST, winres.ID(1), winres.LCIDDefault, manifest); err != nil {
		return nil, err
	}
	return rs, nil
}

var versionPattern = regexp.MustCompile(`^([0-9]+)\.([0-9]+)\.([0-9]+)(?:[-+][0-9A-Za-z.+-]+)?$`)
var copyrightPattern = regexp.MustCompile(`(?m)^Copyright \(c\) [0-9]{4} (.+)\r?$`)

func versionInfo(root string, app application, release string) (*version.Info, error) {
	parts := versionPattern.FindStringSubmatch(release)
	if parts == nil {
		return nil, fmt.Errorf("version %q must have major.minor.patch with an optional prerelease/build suffix", release)
	}
	var fixed [4]uint16
	for i, part := range parts[1:] {
		n, err := strconv.ParseUint(part, 10, 16)
		if err != nil {
			return nil, fmt.Errorf("version component %q exceeds Windows' 16-bit range: %w", part, err)
		}
		fixed[i] = uint16(n)
	}
	license, err := os.ReadFile(filepath.Join(root, "LICENSE"))
	if err != nil {
		return nil, err
	}
	copyright := copyrightPattern.FindStringSubmatch(string(license))
	if copyright == nil {
		return nil, fmt.Errorf("LICENSE has no copyright holder")
	}
	vi := &version.Info{FileVersion: fixed, ProductVersion: fixed}
	vi.Flags.Prerelease = strings.Contains(release, "-")
	for key, value := range map[string]string{
		version.ProductName: "Obituary 讣告", version.FileDescription: app.description,
		version.CompanyName: strings.TrimSpace(copyright[1]), version.LegalCopyright: strings.TrimSpace(copyright[0]),
		version.FileVersion: release, version.ProductVersion: release,
		version.InternalName: app.name, version.OriginalFilename: app.name + ".exe",
	} {
		if err := vi.Set(version.LangDefault, key, value); err != nil {
			return nil, err
		}
	}
	return vi, nil
}
