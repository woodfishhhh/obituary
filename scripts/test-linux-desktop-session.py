"""Observe the packaged SNI/notification protocol inside xvfb-run + dbus-run-session."""
import json
import os
from pathlib import Path
import subprocess
import sys
import time

import dbus
import dbus.service
from dbus.mainloop.glib import DBusGMainLoop
from gi.repository import GLib

DBusGMainLoop(set_as_default=True)
bus = dbus.SessionBus()
work = Path(os.environ['HOME'])
evidence = Path(os.environ['EVIDENCE'])
watcher = 'org.kde.StatusNotifierWatcher'
notifications = 'org.freedesktop.Notifications'


class Watcher(dbus.service.Object):
    @dbus.service.method(watcher, in_signature='s', sender_keyword='sender')
    def RegisterStatusNotifierItem(self, path, sender=None):
        (work / 'registered').write_text(json.dumps([str(sender), str(path)]))


class Notifications(dbus.service.Object):
    @dbus.service.method(notifications, in_signature='susssasa{sv}i', out_signature='u')
    def Notify(self, app, replaces, icon, title, body, actions, hints, timeout):
        (evidence / 'notification.json').write_text(json.dumps(dict(app=app, title=title, body=body)))
        return 1


def until(read):
    for _ in range(100):
        result = read()
        if result:
            return result
        time.sleep(0.1)
    raise RuntimeError(f'timed out: {read}')


def probe():
    sender, path = json.loads(until(lambda: (work / 'registered').exists() and (work / 'registered').read_text()))
    obj = bus.get_object(sender, path)
    assert not bool(obj.Get('org.kde.StatusNotifierItem', 'ItemIsMenu', dbus_interface='org.freedesktop.DBus.Properties'))
    menu_path = obj.Get('org.kde.StatusNotifierItem', 'Menu', dbus_interface='org.freedesktop.DBus.Properties')
    menu = dbus.Interface(bus.get_object(sender, menu_path), 'com.canonical.dbusmenu')

    def rows():
        def walk(node, depth):
            item, props, children = node
            if props.get('visible', True):
                yield int(item), depth, str(props.get('label', ''))
                for child in children:
                    yield from walk(child, depth + 1)
        return list(walk(menu.GetLayout(0, -1, [])[1], 0))

    def populated():
        r = rows()
        return r if any('restore-note' in label for _, _, label in r) else None

    layout = until(populated)
    labels = [s for _, _, s in layout]
    for name in ['Open Obituary', 'Recently buried', 'Expiring soon', 'Pause', 'Open data directory', 'Quit']:
        assert any(label.startswith(name) for label in labels), (name, labels)
    (evidence / 'registered-menu.json').write_text(json.dumps(dict(sender=sender, item=path, menu=menu_path, rows=layout), indent=2))
    # A private GTK observer renders the actual menu labels; this is not a GNOME panel screenshot.
    import gi
    gi.require_version('Gtk', '3.0')
    from gi.repository import Gtk
    window = Gtk.Window(title='StatusNotifierItem menu observer')
    box = Gtk.Box(orientation=Gtk.Orientation.VERTICAL, spacing=4, margin=12)
    window.add(box)
    for _, depth, label in layout:
        if label:
            box.pack_start(Gtk.Label(label='    ' * (depth - 1) + label, xalign=0), False, False, 0)
    window.show_all()
    while Gtk.events_pending():
        Gtk.main_iteration()
    subprocess.run(['import', '-window', str(window.get_window().get_xid()), str(evidence / 'menu-observer.png')], check=True)
    def click(part):
        item = next(i for i, _, label in rows() if part in label)
        menu.Event(item, 'clicked', dbus.Int32(0, variant_level=1), dbus.UInt32(0))
    click('restore-note')
    restored = until(lambda: (work / 'restore-note').exists() and (work / 'restore-note').read_text())
    assert restored.strip() == 'recoverable fixture'
    notice = json.loads(until(lambda: (evidence / 'notification.json').exists() and (evidence / 'notification.json').read_text()))
    assert notice['title'] == 'Resurrected' and 'restore-note' in notice['body'], notice
    click('Pause')
    until(lambda: any(label == 'Resume' for _, _, label in rows()))
    click('Resume')
    until(lambda: any(label == 'Pause' for _, _, label in rows()))
    try:
        click('Quit')
    except dbus.exceptions.DBusException as exc:
        assert exc.get_dbus_name() == 'org.freedesktop.DBus.Error.NoReply', exc
    print('PASS: SNI menu, restore, notification and subsequent pause/resume')


if __name__ == '__main__':
    if sys.argv[1] == 'host':
        host = Watcher(dbus.service.BusName(watcher, bus), '/StatusNotifierWatcher')
        service = Notifications(dbus.service.BusName(notifications, bus), '/org/freedesktop/Notifications')
        GLib.MainLoop().run()
    else:
        probe()
