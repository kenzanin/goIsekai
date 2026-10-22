// Top navigation, mirroring the web UI's partials/nav.lua: the brand doubles as
// the library (home) tab, then one tab per top-level page.

import QtQuick
import QtQuick.Controls
import QtQuick.Layouts

// Only the library page has content today; the rest are full pages (not
// dialogs) so adding one later means filling in a page rather than reshaping
// the shell.
Rectangle {
    id: nav

    property string currentTab: "library"
    readonly property var tabs: [{
        "key": "library",
        "label": "goIsekai"
    }, {
        "key": "search",
        "label": "Search"
    }, {
        "key": "updates",
        "label": "Updates"
    }, {
        "key": "history",
        "label": "History"
    }, {
        "key": "plugins",
        "label": "Plugins"
    }, {
        "key": "settings",
        "label": "Settings"
    }, {
        "key": "logs",
        "label": "Logs"
    }, {
        "key": "about",
        "label": "About"
    }]

    signal tabSelected(string tab)

    implicitHeight: 48
    color: Theme.panel

    RowLayout {
        anchors.fill: parent
        anchors.leftMargin: Theme.gap
        anchors.rightMargin: Theme.gap
        spacing: 4

        Repeater {
            model: nav.tabs

            delegate: Rectangle {
                required property var modelData
                readonly property bool on: modelData.key === nav.currentTab
                // The brand is the home tab, so it keeps its weight when
                // active rather than turning into an ordinary chip.
                readonly property bool brand: modelData.key === "library"

                Layout.preferredHeight: 34
                Layout.alignment: Qt.AlignVCenter
                implicitWidth: label.implicitWidth + (brand ? 28 : 24)
                radius: 6
                color: on ? "#3a2a10" : "transparent"

                Label {
                    id: label

                    anchors.centerIn: parent
                    text: modelData.label
                    color: parent.on ? Theme.accent : (parent.brand ? Theme.text : Theme.textDim)
                    font.pixelSize: 17
                    font.bold: parent.brand
                }

                // Underline marks the active tab the way the web nav's
                // border-b-2 does.
                Rectangle {
                    visible: parent.on
                    anchors.bottom: parent.bottom
                    anchors.left: parent.left
                    anchors.right: parent.right
                    anchors.margins: 4
                    height: 2
                    radius: 1
                    color: Theme.accent
                }

                MouseArea {
                    anchors.fill: parent
                    cursorShape: Qt.PointingHandCursor
                    onClicked: nav.tabSelected(modelData.key)
                }

            }

        }

        Item {
            Layout.fillWidth: true
        }

    }

}
