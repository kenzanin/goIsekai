import QtQuick
pragma Singleton

// Palette copied from the web UI's compiled Tailwind classes so both front ends
// look like one product. The accent is indigo (#6366f1), not the orange used by
// the old miqt client.
QtObject {
    // indigo-950, active tab wash
    // emerald-600

    // Surfaces: neutral-900 / neutral-800 / neutral-700.
    readonly property color bg: "#171717"
    readonly property color panel: "#171717"
    readonly property color card: "#171717"
    readonly property color field: "#171717"
    readonly property color line: "#262626"
    readonly property color lineStrong: "#404040"
    // Indigo accent ramp.
    readonly property color accent: "#6366f1"
    // indigo-500
    readonly property color accentStrong: "#4f46e5"
    // indigo-600
    readonly property color accentSoft: "#818cf8"
    // indigo-400
    readonly property color accentText: "#a5b4fc"
    // indigo-300
    readonly property color accentWash: "#1e1b4b"
    // Text: neutral-200 / 400 / 500.
    readonly property color text: "#e5e5e5"
    readonly property color textDim: "#a3a3a3"
    readonly property color textFaint: "#737373"
    // Pills from the web cards.
    readonly property color newBadge: "#ef4444"
    // red-500
    readonly property color doneBadge: "#059669"
    readonly property int radius: 8
    readonly property int gap: 16
    // Fallback cover aspect ratio; plugins may declare their own thumb_ratio.
    readonly property real defaultRatio: 0.703
    // The web root font is 17.5px, so its text-xs lands near 13px. The floor
    // here is 15px for every size, per requirement.
    readonly property int fsSmall: 15
    readonly property int fsBody: 16
    readonly property int fsTitle: 18
    readonly property int fsHeading: 22
}
