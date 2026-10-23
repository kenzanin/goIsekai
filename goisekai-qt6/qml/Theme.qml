import QtQuick
pragma Singleton

// Palette shared with the web UI, so both front ends look like one product.
QtObject {
    readonly property color bg: "#171717"
    readonly property color panel: "#1f1f1f"
    readonly property color card: "#242424"
    readonly property color field: "#2a2a2a"
    readonly property color line: "#3a3a3a"
    readonly property color accent: "#ffa332"
    readonly property color text: "#e5e5e5"
    readonly property color textDim: "#9ca3af"
    readonly property color textFaint: "#6b7280"
    readonly property int radius: 8
    readonly property int gap: 16
    readonly property real defaultRatio: 0.703
}
