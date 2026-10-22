import ".."
import QtQuick
import QtQuick.Controls
import QtQuick.Layouts

// Statistics, plugin counts, tag counts, duplicate count. Clicking an active row
// clears that filter, so there is always a way back.
Rectangle {
    id: side

    property var facets: ({
    })
    property string pluginFilter: ""
    property string tagFilter: ""
    property bool onlyDuplicates: false
    readonly property var overview: facets.overview || ({
    })
    readonly property var plugins: facets.plugins || []
    readonly property var categories: facets.categories || []
    readonly property int duplicateCount: facets.duplicate_count || 0

    signal pluginToggled(string pluginID)
    signal tagToggled(string tag)
    signal duplicatesToggled()
    signal anyFilterCleared()

    color: Theme.panel

    ScrollView {
        anchors.fill: parent
        anchors.margins: 12
        clip: true
        ScrollBar.horizontal.policy: ScrollBar.AlwaysOff

        ColumnLayout {
            width: side.width - 24
            spacing: 14

            // ── Statistics ──────────────────────────────────────────────────
            Label {
                text: "LIBRARY"
                color: Theme.textFaint
                font.pixelSize: 12
                font.bold: true
            }

            GridLayout {
                columns: 2
                columnSpacing: 8
                rowSpacing: 8
                Layout.fillWidth: true

                Repeater {
                    model: [{
                        "v": side.overview.total_titles || 0,
                        "l": "titles"
                    }, {
                        "v": side.overview.fully_read || 0,
                        "l": "finished"
                    }, {
                        "v": side.overview.started_reading || 0,
                        "l": "reading"
                    }, {
                        "v": side.overview.has_updates || 0,
                        "l": "updates"
                    }]

                    delegate: Rectangle {
                        required property var modelData

                        Layout.fillWidth: true
                        implicitHeight: 56
                        radius: 6
                        color: Theme.card

                        Column {
                            anchors.centerIn: parent

                            Label {
                                text: modelData.v
                                color: Theme.accent
                                font.pixelSize: 18
                                font.bold: true
                                anchors.horizontalCenter: parent.horizontalCenter
                            }

                            Label {
                                text: modelData.l
                                color: Theme.textFaint
                                font.pixelSize: 12
                                anchors.horizontalCenter: parent.horizontalCenter
                            }

                        }

                    }

                }

            }

            Rectangle {
                visible: side.duplicateCount > 0
                Layout.fillWidth: true
                implicitHeight: 38
                radius: 6
                color: side.onlyDuplicates ? "#3a2a10" : Theme.card
                border.width: 1
                border.color: side.onlyDuplicates ? Theme.accent : "transparent"

                Label {
                    anchors.centerIn: parent
                    text: side.duplicateCount + " in duplicate groups"
                    color: side.onlyDuplicates ? Theme.accent : Theme.textDim
                    font.pixelSize: 13
                }

                MouseArea {
                    anchors.fill: parent
                    cursorShape: Qt.PointingHandCursor
                    onClicked: side.duplicatesToggled()
                }

            }

            // ── Plugins ─────────────────────────────────────────────────────
            RowLayout {
                Layout.fillWidth: true

                Label {
                    text: "SOURCES"
                    color: Theme.textFaint
                    font.pixelSize: 12
                    font.bold: true
                }

                Item {
                    Layout.fillWidth: true
                }

                Label {
                    visible: side.pluginFilter !== ""
                    text: "clear"
                    color: Theme.accent
                    font.pixelSize: 12

                    MouseArea {
                        anchors.fill: parent
                        cursorShape: Qt.PointingHandCursor
                        onClicked: side.anyFilterCleared()
                    }

                }

            }

            ColumnLayout {
                Layout.fillWidth: true
                spacing: 2

                Repeater {
                    model: side.plugins

                    delegate: Rectangle {
                        required property var modelData
                        readonly property bool on: modelData.plugin_id === side.pluginFilter

                        Layout.fillWidth: true
                        implicitHeight: 30
                        radius: 4
                        color: on ? "#3a2a10" : "transparent"

                        RowLayout {
                            anchors.fill: parent
                            anchors.leftMargin: 4
                            anchors.rightMargin: 4
                            spacing: 6

                            Image {
                                source: modelData.icon ? (modelData.icon.indexOf("data:") === 0 ? modelData.icon : serverBase + modelData.icon) : ""
                                sourceSize.width: 16
                                sourceSize.height: 16
                                asynchronous: true
                                visible: modelData.icon !== ""
                            }

                            Label {
                                Layout.fillWidth: true
                                text: modelData.name
                                elide: Text.ElideRight
                                color: on ? Theme.accent : Theme.textDim
                                font.pixelSize: 13
                            }

                            Label {
                                text: modelData.count
                                color: Theme.textFaint
                                font.pixelSize: 12
                            }

                        }

                        MouseArea {
                            anchors.fill: parent
                            cursorShape: Qt.PointingHandCursor
                            onClicked: side.pluginToggled(modelData.plugin_id)
                        }

                    }

                }

            }

            // ── Tags ────────────────────────────────────────────────────────
            RowLayout {
                Layout.fillWidth: true

                Label {
                    text: "TAGS"
                    color: Theme.textFaint
                    font.pixelSize: 12
                    font.bold: true
                }

                Item {
                    Layout.fillWidth: true
                }

                Label {
                    visible: side.tagFilter !== ""
                    text: side.tagFilter
                    color: Theme.accent
                    font.pixelSize: 12

                    MouseArea {
                        anchors.fill: parent
                        cursorShape: Qt.PointingHandCursor
                        onClicked: side.tagToggled(side.tagFilter)
                    }

                }

            }

            Flow {
                Layout.fillWidth: true
                spacing: 4

                Repeater {
                    model: side.categories

                    delegate: Rectangle {
                        required property var modelData
                        readonly property bool on: modelData.name === side.tagFilter

                        width: chip.implicitWidth + 14
                        height: 26
                        radius: 4
                        color: on ? "#3a2a10" : Theme.card
                        border.width: 1
                        border.color: on ? Theme.accent : "transparent"

                        Label {
                            id: chip

                            anchors.centerIn: parent
                            text: modelData.name + "  " + modelData.count
                            color: on ? Theme.accent : Theme.textDim
                            font.pixelSize: 12
                        }

                        MouseArea {
                            anchors.fill: parent
                            cursorShape: Qt.PointingHandCursor
                            onClicked: side.tagToggled(modelData.name)
                        }

                    }

                }

            }

            Item {
                Layout.fillHeight: true
            }

        }

    }

}
