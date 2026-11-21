// Use only the frontmost Obituary window on a clean acceptance runner. Some
// WebKit builds display the controls without exposing AXPress to System Events.
import AppKit
import Vision

let args = CommandLine.arguments
guard NSWorkspace.shared.frontmostApplication?.bundleIdentifier == "site.woodfish.obituary.gui",
      let windows = CGWindowListCopyWindowInfo([.optionOnScreenOnly, .excludeDesktopElements], kCGNullWindowID) as? [[String: Any]],
      let win = windows.first(where: { ($0[kCGWindowOwnerPID as String] as? pid_t) == NSWorkspace.shared.frontmostApplication?.processIdentifier && ($0[kCGWindowLayer as String] as? Int) == 0 }),
      let bounds = win[kCGWindowBounds as String] as? NSDictionary,
      let box = CGRect(dictionaryRepresentation: bounds) else { exit(2) }
let screen = CGDisplayBounds(CGMainDisplayID())
let req = VNRecognizeTextRequest()
try VNImageRequestHandler(url: URL(fileURLWithPath: args[1])).perform([req])
let settings = args[2] == "Settings"
for line in req.results ?? [] {
    guard let text = line.topCandidates(1).first?.string else { continue }
    let rect = line.boundingBox
    var pt = CGPoint(x: rect.midX * screen.width, y: (1 - rect.midY) * screen.height)
    guard box.contains(pt) else { continue }
    if settings ? text == "Settings" : text.hasPrefix("Start the tray at sign") || text.hasPrefix("Start the tray when you sign") {
        if !settings { pt.x = box.maxX - 65 }
        print("click \(text) at \(pt) in \(box)")
        for kind in [CGEventType.leftMouseDown, .leftMouseUp] {
            CGEvent(mouseEventSource: nil, mouseType: kind, mouseCursorPosition: pt, mouseButton: .left)?.post(tap: .cghidEventTap)
        }
        exit(0)
    }
}
if !settings {
    CGWarpMouseCursorPosition(CGPoint(x: box.midX, y: box.midY))
    CGEvent(scrollWheelEvent2Source: nil, units: .pixel, wheelCount: 1, wheel1: -230, wheel2: 0, wheel3: 0)?.post(tap: .cghidEventTap)
}
exit(1)
