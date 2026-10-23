import QtQuick
import QtQuick.Controls
import QtQuick.Layouts
import Goisekai

// Window shell. Owns only the nav bar and which tab is active; every page is a
// self-contained component (LibraryPage, …) that keeps its own state, so adding
// a new page means dropping one file here — not growing this one.
ApplicationWindow {
    id: root

    property string currentTab: "library"
    // pluginId:sourceMangaId of the manga shown on the detail page, set by
    // LibraryPage's detailRequested signal and passed to MangaDetailPage.qml
    // which fetches it itself.
    property string detailKey: ""
    // Forwarded from LibraryPage so MangaDetailPage can resolve plugin labels.
    property var _libPluginNames: ({})
    readonly property var navTabs: [
        { "key": "library", "label": "goIsekai" },
        { "key": "search", "label": "Search" },
        { "key": "updates", "label": "Updates" },
        { "key": "history", "label": "History" },
        { "key": "plugins", "label": "Plugins" },
        { "key": "settings", "label": "Settings" },
        { "key": "logs", "label": "Logs" },
        { "key": "about", "label": "About" }
    ]

    visible: true
    width: 1280
    height: 860
    minimumWidth: 720
    minimumHeight: 480
    title: "goIsekai"
    color: Theme.bg

    // Nav bar: always visible, on every tab (mirrors partials/nav.lua).
    header: Rectangle {
        implicitHeight: 52
        color: Theme.panel

        Rectangle {
            anchors.bottom: parent.bottom
            anchors.left: parent.left
            anchors.right: parent.right
            height: 1
            color: Theme.line
        }

        RowLayout {
            anchors.fill: parent
            anchors.leftMargin: Theme.gap
            anchors.rightMargin: Theme.gap
            spacing: 4

            Repeater {
                model: root.navTabs

                delegate: Rectangle {
                    required property var modelData
                    readonly property bool on: modelData.key === root.currentTab
                    readonly property bool brand: modelData.key === "library"

                    Layout.preferredHeight: 36
                    Layout.alignment: Qt.AlignVCenter
                    implicitWidth: navLabel.implicitWidth + (brand ? 28 : 24)
                    radius: 6
                    color: on ? Theme.accentWash : (navMouse.containsMouse ? Theme.field : "transparent")

                    Label {
                        id: navLabel
                        anchors.centerIn: parent
                        text: modelData.label
                        color: parent.on ? Theme.accentText
                                         : (parent.brand ? Theme.text : Theme.textDim)
                        font.pixelSize: Theme.fsBody
                        font.bold: parent.brand
                    }

                    // Active underline, like the web nav's border-b-2.
                    Rectangle {
                        visible: parent.on
                        anchors.bottom: parent.bottom
                        anchors.left: parent.left
                        anchors.right: parent.right
                        height: 2
                        color: Theme.accentSoft
                    }

                    MouseArea {
                        id: navMouse
                        anchors.fill: parent
                        hoverEnabled: true
                        cursorShape: Qt.PointingHandCursor
                        onClicked: root.currentTab = modelData.key
                    }
                }
            }

            Item { Layout.fillWidth: true }
        }
    }

    // ---- Tab content ------------------------------------------------------
    // Only the library page exists so far; every other tab renders a
    // placeholder so the nav is real without pretending pages exist.
    LibraryPage {
        anchors.fill: parent
        visible: root.currentTab === "library"
        // Library -> detail page navigation (in-process).
        onDetailRequested: {
            root.detailKey = key;
            root.currentTab = "detail";
        }
    }

    // Manga detail page. Renders on top of the layout when currentTab is
    // "detail", driven by detailKey (pluginId:sourceMangaId).
    MangaDetailPage {
        anchors.fill: parent
        visible: root.currentTab === "detail"
        detailKey: root.detailKey
        onBackRequested: {
            root.detailKey = "";
            root.currentTab = "library";
        }
    }

    Rectangle {
        anchors.fill: parent
        visible: root.currentTab !== "library" && root.currentTab !== "detail"
        color: Theme.bg

        ColumnLayout {
            anchors.centerIn: parent
            spacing: 8

            Label {
                text: root.currentTab.charAt(0).toUpperCase() + root.currentTab.slice(1)
                color: Theme.textDim
                font.pixelSize: Theme.fsTitle
                font.bold: true
            }
            Label {
                text: "not implemented yet"
                color: Theme.textFaint
                font.pixelSize: Theme.fsBody
            }
        }
    }
}
