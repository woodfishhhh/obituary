package docker

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"time"
)

// VolumeClient is the small Docker surface used by the volume lifecycle.
type VolumeClient interface {
	InspectVolume(context.Context, string) (Volume, error)
	ListVolumeReferences(context.Context, Volume) ([]Container, error)
	ArchiveVolume(context.Context, *Volume, io.Writer) (VolumeArchive, error)
	RemoveVolume(context.Context, *Volume) error
	CreateVolume(context.Context, Volume, string) (Volume, error)
	RestoreVolume(context.Context, Volume, io.Reader) error
}

var ErrVolumeReferenced = errors.New("Docker volume is referenced by a container")

func (c CLI) ListVolumeReferences(ctx context.Context, want Volume) ([]Container, error) {
	bound, err := c.bind(WithTarget(ctx, &want.Target))
	if err != nil {
		return nil, err
	}
	rows, err := bound.List(ctx)
	if err != nil {
		return nil, err
	}
	var refs []Container
	for _, row := range rows {
		used, err := volumeReference(row, want.Name, want.mountpoint)
		if err != nil {
			return nil, err
		}
		if used {
			refs = append(refs, row)
		}
	}
	return refs, nil
}

func (c CLI) ArchiveVolume(ctx context.Context, want *Volume, out io.Writer) (VolumeArchive, error) {
	bound, err := c.bind(WithTarget(ctx, &want.Target))
	if err != nil {
		return VolumeArchive{}, err
	}
	got, err := bound.InspectVolume(ctx, want.Name)
	if err != nil {
		return VolumeArchive{}, err
	}
	if !got.Same(*want) {
		return VolumeArchive{}, ErrIdentity
	}
	refs, err := bound.ListVolumeReferences(ctx, got)
	if err != nil {
		return VolumeArchive{}, err
	}
	if len(refs) != 0 {
		return VolumeArchive{}, fmt.Errorf("%w: %s", ErrVolumeReferenced, refs[0].Display())
	}
	if err := bound.createVolumeHolder(ctx, want); err != nil {
		return VolumeArchive{}, err
	}
	archive, err := bound.archiveStream(ctx, got, out)
	if err != nil {
		_, _ = bound.run(ctx, "rm", "--", want.HolderID)
		want.HolderID = ""
		return VolumeArchive{}, err
	}
	return archive, nil
}

func (c CLI) RemoveVolume(ctx context.Context, want *Volume) error {
	bound, err := c.bind(WithTarget(ctx, &want.Target))
	if err != nil {
		return err
	}
	got, err := bound.InspectVolume(ctx, want.Name)
	if err != nil {
		return err
	}
	if !got.Same(*want) {
		return ErrIdentity
	}
	refs, err := bound.ListVolumeReferences(ctx, got)
	if err != nil {
		return err
	}
	for i := range refs {
		if refs[i].ID == want.HolderID {
			refs = append(refs[:i], refs[i+1:]...)
			break
		}
	}
	if len(refs) != 0 {
		return fmt.Errorf("%w: %s", ErrVolumeReferenced, refs[0].Display())
	}
	if want.HolderID != "" {
		if _, err := bound.run(ctx, "rm", "--", want.HolderID); err != nil {
			return err
		}
		want.HolderID = ""
	}
	_, err = bound.run(ctx, "volume", "rm", "--", want.Name)
	return err
}

func (c CLI) CreateVolume(ctx context.Context, want Volume, token string) (Volume, error) {
	bound, err := c.bind(WithTarget(ctx, &want.Target))
	if err != nil {
		return Volume{}, err
	}
	labels, err := want.RestoreLabels(nil, token)
	if err != nil {
		return Volume{}, err
	}
	args := []string{"volume", "create", "--driver", "local"}
	for key, value := range labels {
		args = append(args, "--label", key+"="+value)
	}
	args = append(args, "--", want.Name)
	if _, err := bound.run(ctx, args...); err != nil {
		return Volume{}, err
	}
	got, err := bound.InspectVolume(ctx, want.Name)
	if err != nil {
		return Volume{}, err
	}
	if got.Driver != want.Driver || got.Scope != want.Scope || !sameLabels(got.Labels, labels) {
		return Volume{}, ErrIdentity
	}
	got.RestoreLabelOwned = true
	return got, nil
}

func (c CLI) createVolumeHolder(ctx context.Context, want *Volume) error {
	label := "io.obituary.operation=" + randomOperationToken()
	mount := "type=volume,source=" + want.Name + ",destination=/volume,volume-nocopy"
	out, err := c.run(ctx, "create", "--label", label, "--mount", mount, volumeImage, "sleep", "3600")
	if err != nil {
		return err
	}
	want.HolderID = strings.TrimSpace(string(out))
	if want.HolderID == "" {
		return errors.New("Docker placeholder container returned no ID")
	}
	return nil
}

func randomOperationToken() string {
	var raw [16]byte
	if _, err := io.ReadFull(rand.Reader, raw[:]); err != nil {
		return hex.EncodeToString(raw[:])
	}
	return hex.EncodeToString(raw[:])
}

func (c CLI) RestoreVolume(ctx context.Context, want Volume, archive io.Reader) error {
	bound, err := c.bind(WithTarget(ctx, &want.Target))
	if err != nil {
		return err
	}
	_, err = bound.streamVolume(ctx, want, archive, true)
	return err
}

func sameLabels(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	for key, value := range b {
		if a[key] != value {
			return false
		}
	}
	return true
}

func (c CLI) archiveStream(ctx context.Context, want Volume, out io.Writer) (VolumeArchive, error) {
	mount := "type=volume,source=" + want.Name + ",destination=/volume,readonly,volume-nocopy"
	args := []string{"run", "--rm", "--network", "none", "--read-only", "--mount", mount, volumeImage, "tar", "--format=pax", "--numeric-owner", "--acls", "--xattrs", "--sparse", "-cpf", "-", "-C", "/volume", "."}
	digest := sha256.New()
	var bytesWritten int64
	if err := c.runStream(ctx, nil, io.MultiWriter(out, digest, counterWriter{n: &bytesWritten}), args...); err != nil {
		return VolumeArchive{}, err
	}
	return VolumeArchive{Digest: hex.EncodeToString(digest.Sum(nil)), Bytes: bytesWritten}, nil
}

func (c CLI) streamVolume(ctx context.Context, want Volume, archive io.Reader, restore bool) (VolumeArchive, error) {
	mount := "type=volume,source=" + want.Name + ",destination=/volume,volume-nocopy"
	args := []string{"run", "--rm", "-i", "--network", "none", "--read-only", "--mount", mount, volumeImage, "tar", "--numeric-owner", "--acls", "--xattrs", "--sparse", "-xpf", "-", "-C", "/volume"}
	if err := c.runStream(ctx, archive, nil, args...); err != nil {
		return VolumeArchive{}, err
	}
	return VolumeArchive{}, nil
}

func (c CLI) runStream(ctx context.Context, input io.Reader, output io.Writer, args ...string) error {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()
	executable := c.Executable
	if executable == "" {
		executable = "docker"
	}
	cmd := exec.CommandContext(ctx, executable, c.arguments(args)...)
	if c.target != nil {
		cmd.Env = targetEnvironment()
	}
	hideWindow(cmd)
	cmd.WaitDelay = time.Second
	cmd.Stdin, cmd.Stdout = input, output
	var stderr limitedBuffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil {
			return fmt.Errorf("docker %s: %w: %w", args[0], ErrUnavailable, ctx.Err())
		}
		return fmt.Errorf("docker %s: %w: %w%s", args[0], ErrUnavailable, err, stderr.String())
	}
	return nil
}

type counterWriter struct{ n *int64 }

func (w counterWriter) Write(p []byte) (int, error) { *w.n += int64(len(p)); return len(p), nil }

type limitedBuffer struct {
	data []byte
}

func (b *limitedBuffer) Write(p []byte) (int, error) {
	if len(b.data) < 8192 {
		b.data = append(b.data, p[:min(len(p), 8192-len(b.data))]...)
	}
	return len(p), nil
}

func (b *limitedBuffer) String() string { return strings.TrimSpace(string(b.data)) }

func archiveDigest(r io.Reader, out io.Writer) (VolumeArchive, error) {
	hash := sha256.New()
	n, err := io.Copy(io.MultiWriter(out, hash), r)
	if err != nil {
		return VolumeArchive{}, err
	}
	return VolumeArchive{Digest: hex.EncodeToString(hash.Sum(nil)), Bytes: n}, nil
}
