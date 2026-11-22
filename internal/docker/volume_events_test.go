package docker

import "testing"

func TestVolumeEvidenceRequiresAnchorsAndNoForeignChanges(t *testing.T) {
	holder, helper, witness := "holder", "helper", "witness"
	evt := func(kind, action, id, container string, ts int64) volumeEvent {
		var e volumeEvent
		e.Type, e.Action, e.Actor.ID, e.TimeNano = kind, action, id, ts
		e.Actor.Attributes = map[string]string{"container": container}
		return e
	}
	start := evt("container", "create", holder, "", 1)
	read := evt("volume", "mount", "cache", helper, 2)
	end := evt("container", "destroy", helper, "", 5)
	released := evt("container", "destroy", holder, "", 6)
	removed := evt("volume", "destroy", "cache", "", 7)
	finished := evt("container", "destroy", witness, "", 9)
	proof := volumeEvidence{Name: "cache", Holder: holder, Helper: helper, Witness: witness}
	for _, tc := range []struct {
		name     string
		rows     []volumeEvent
		post, ok bool
	}{
		{"pre", []volumeEvent{start, read, end}, false, true}, {"post", []volumeEvent{start, read, end, released, removed, finished}, true, true},
		{"foreign-mount-already-in-preflight-response", []volumeEvent{start, read, end, evt("volume", "mount", "cache", "foreign", 6)}, false, false},
		{"holder-lost-before-removal", []volumeEvent{start, read, end, released}, false, false},
		{"unconfirmed-holder-release", []volumeEvent{start, read, end, removed, finished}, true, false},
		{"truncated", []volumeEvent{read, end}, false, false}, {"missing-own-read", []volumeEvent{start, end}, false, false}, {"no-final-fence", []volumeEvent{start, read, end, removed}, true, false},
		{"concurrent-writer", []volumeEvent{start, read, evt("volume", "mount", "cache", "foreign", 3), end}, false, false},
		{"unknown-container-created", []volumeEvent{start, read, evt("container", "create", "foreign", "", 3), end}, false, false},
		{"replacement", []volumeEvent{start, read, end, released, removed, evt("volume", "create", "cache", "", 8), finished}, true, false},
		{"foreign-volume", []volumeEvent{start, read, evt("volume", "mount", "other", "foreign", 3), end}, false, true},
		{"clock-backwards", []volumeEvent{start, read, evt("volume", "unmount", "cache", helper, 1), end}, false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := proof.check(tc.rows, tc.post); (err == nil) != tc.ok {
				t.Fatalf("err=%v", err)
			}
		})
	}
}

func TestCheckVolumeEventsRejectsMissingAndForeignActivity(t *testing.T) {
	makeEvent := func(kind, action, id, container string, ts int64) volumeEvent {
		var row volumeEvent
		row.Type, row.Action, row.TimeNano, row.Actor.ID = kind, action, ts, id
		row.Actor.Attributes = map[string]string{"container": container}
		return row
	}
	pre := []volumeEvent{
		makeEvent("container", "create", "holder", "", 1),
		makeEvent("volume", "mount", "cache", "helper", 2),
		makeEvent("container", "destroy", "helper", "", 3),
	}
	if err := CheckVolumeEvents(pre, "cache", "holder", false); err != nil {
		t.Fatal(err)
	}
	post := append(pre, makeEvent("container", "destroy", "holder", "", 4), makeEvent("volume", "destroy", "cache", "", 5))
	if err := CheckVolumeEvents(post, "cache", "holder", true); err != nil {
		t.Fatal(err)
	}
	foreign := append(pre, makeEvent("volume", "mount", "cache", "foreign", 4))
	if err := CheckVolumeEvents(foreign, "cache", "holder", false); err == nil {
		t.Fatal("foreign volume activity accepted")
	}
}
